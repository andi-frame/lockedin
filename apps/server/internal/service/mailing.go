package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// Email tasks can run more than once (asynq retries, worker restarts). Each Claim* below marks
// the row as emailed in the same statement that reads it, so only one run gets the data and a
// retry gets "nothing to send". The caller gives the claim back with Release* when the send
// fails. A crash between claim and send loses that one email; the in-app notification is
// unaffected. We accept that over sending a message twice.

// NotificationEmail is what the mailer needs to write one notification's email.
type NotificationEmail struct {
	NotificationID int64
	To             string
	ToName         string
	Locale         string
	Kind           string
	PactID         uuid.UUID
	PactTitle      string
}

// ClaimNotificationEmail returns the email data for a notification, or ok=false when there is
// nothing to send (already claimed or sent, the notification no longer exists, or the person has
// switched this kind of email off).
func (s *Service) ClaimNotificationEmail(ctx context.Context, id int64) (NotificationEmail, bool, error) {
	var out NotificationEmail
	var found bool
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		row, err := q.ClaimNotificationEmail(ctx, id)
		if store.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		// Claimed either way: a switched-off kind must not come back on a retry (PLAN 9.4).
		if !wantsEmail(row.Kind, row.EmailOff) {
			return nil
		}
		var n Notification
		if err := json.Unmarshal(row.Payload, &n); err != nil || n.PactID == uuid.Nil {
			return fmt.Errorf("notification %d has no pact: %w", id, errors.Join(err, errUnreadablePayload))
		}
		p, err := q.GetPact(ctx, n.PactID)
		if err != nil {
			return err
		}
		out = NotificationEmail{
			NotificationID: row.ID, To: row.Email, ToName: row.DisplayName, Locale: row.Locale,
			Kind: row.Kind, PactID: n.PactID, PactTitle: p.Title,
		}
		found = true
		return nil
	})
	return out, found && err == nil, err
}

var errUnreadablePayload = errors.New("unreadable notification payload")

// ReleaseNotificationEmail gives a claim back after a failed send.
func (s *Service) ReleaseNotificationEmail(ctx context.Context, id int64) error {
	return s.st.ReleaseNotificationEmail(ctx, []int64{id})
}

// DigestEmail is one email that covers every unsent notification of a kind for a user and pact.
type DigestEmail struct {
	IDs       []int64
	To        string
	ToName    string
	Locale    string
	Kind      string
	PactID    uuid.UUID
	PactTitle string
	Count     int
}

// ClaimDigestEmail claims all unsent notifications of one kind for the user in the pact, so a
// burst of events (SPEC §9: "proof submitted ... email digest") becomes one message.
func (s *Service) ClaimDigestEmail(ctx context.Context, user, pact uuid.UUID, kind string) (DigestEmail, bool, error) {
	var out DigestEmail
	var found bool
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		ids, err := q.ClaimDigestNotifications(ctx, store.ClaimDigestNotificationsParams{UserID: user, Kind: kind, PactID: pact.String()})
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			return nil
		}
		u, err := q.GetUser(ctx, user)
		if err != nil {
			return err
		}
		if !wantsEmail(kind, u.EmailOff) {
			return nil // claimed above, so these are never mailed (and never retried)
		}
		p, err := q.GetPact(ctx, pact)
		if err != nil {
			return err
		}
		out = DigestEmail{IDs: ids, To: u.Email, ToName: u.DisplayName, Locale: u.Locale, Kind: kind, PactID: pact, PactTitle: p.Title, Count: len(ids)}
		found = true
		return nil
	})
	return out, found && err == nil, err
}

// ReleaseDigestEmail gives a digest claim back after a failed send.
func (s *Service) ReleaseDigestEmail(ctx context.Context, ids []int64) error {
	return s.st.ReleaseNotificationEmail(ctx, ids)
}

// InviteEmail is what the mailer needs to write an invite. Token is the same plaintext the
// caller passed in; it is echoed so the mailer has one value to build the link from.
type InviteEmail struct {
	To         string
	PactTitle  string
	BackerName string
	ExpiresAt  time.Time
	Token      string
}

// ClaimInviteEmail returns the invite email data, or ok=false when the invite has no address,
// was already mailed, was used, has expired or does not exist.
func (s *Service) ClaimInviteEmail(ctx context.Context, token string) (InviteEmail, bool, error) {
	row, err := s.st.ClaimInviteEmail(ctx, store.ClaimInviteEmailParams{TokenHash: hashToken(token), ExpiresAt: s.clock.Now()})
	if store.IsNoRows(err) {
		return InviteEmail{}, false, nil
	}
	if err != nil {
		return InviteEmail{}, false, err
	}
	return InviteEmail{To: row.Email, PactTitle: row.Title, BackerName: row.BackerName, ExpiresAt: row.ExpiresAt, Token: token}, true, nil
}

// ReleaseInviteEmail gives an invite claim back after a failed send.
func (s *Service) ReleaseInviteEmail(ctx context.Context, token string) error {
	return s.st.ReleaseInviteEmail(ctx, hashToken(token))
}
