package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/andi-frame/lockedin/apps/server/internal/notify"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// Event-driven email tasks. They are enqueued by the outbox relay after its transaction has
// committed (ARCHITECTURE §3) and never appear in Schedule().
const (
	TypeEmailNotification = "email:notification"
	TypeEmailDigest       = "email:digest"
	TypeEmailInvite       = "email:invite"
)

const (
	emailMaxRetry = 5
	emailTimeout  = 45 * time.Second
	// digestWindow is how long proof_submitted notifications gather before one email goes out.
	// Together with Unique it makes a burst of submissions a single message (SPEC §9).
	digestWindow = 5 * time.Minute
)

// Mailing is what the email handlers need from internal/service.
type Mailing interface {
	ClaimNotificationEmail(ctx context.Context, id int64) (service.NotificationEmail, bool, error)
	ReleaseNotificationEmail(ctx context.Context, id int64) error
	ClaimDigestEmail(ctx context.Context, user, pact uuid.UUID, kind string) (service.DigestEmail, bool, error)
	ReleaseDigestEmail(ctx context.Context, ids []int64) error
	ClaimInviteEmail(ctx context.Context, token string) (service.InviteEmail, bool, error)
	ReleaseInviteEmail(ctx context.Context, token string) error
}

// Enqueuer is the part of *asynq.Client the relay uses.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Mail bundles everything the email side needs. Leave Handlers.mail nil to run without email.
type Mail struct {
	Svc      Mailing
	Renderer *notify.Renderer
	Sender   notify.Sender
	Queue    Enqueuer
}

type notificationPayload struct {
	ID int64 `json:"id"`
}

type digestPayload struct {
	UserID uuid.UUID `json:"user_id"`
	PactID uuid.UUID `json:"pact_id"`
	Kind   string    `json:"kind"`
}

type invitePayload struct {
	Token string `json:"token"`
}

// queueEmails enqueues the email tasks for what the relay just delivered. The relay has
// already committed, so a failure here is logged and counted, not returned: the in-app
// notification exists and re-running the relay would find nothing to do.
func (h *Handlers) queueEmails(res service.RelayResult) {
	if h.mail == nil {
		return
	}
	for _, n := range res.Notifications {
		switch notify.DeliveryFor(n.Kind) {
		case notify.Immediate:
			h.enqueue(TypeEmailNotification, notificationPayload{ID: n.ID},
				asynq.TaskID(fmt.Sprintf("email-n-%d", n.ID)))
		case notify.Digest:
			// Unique on (type, payload): while a digest for this user, pact and kind is waiting,
			// further submissions fold into it. A submission that lands while that digest is
			// already being sent waits for the next digest; its in-app notification is unaffected.
			h.enqueue(TypeEmailDigest, digestPayload{UserID: n.UserID, PactID: n.PactID, Kind: n.Kind},
				asynq.ProcessIn(digestWindow), asynq.Unique(digestWindow+emailTimeout))
		}
	}
	for _, inv := range res.Invites {
		sum := sha256.Sum256([]byte(inv.Token))
		// The token travels in the task payload. Retention(0) keeps a finished task from lingering
		// in Redis; the token is single-use and expires with the invite either way.
		h.enqueue(TypeEmailInvite, invitePayload{Token: inv.Token},
			asynq.TaskID("email-i-"+hex.EncodeToString(sum[:8])), asynq.Retention(0))
	}
}

func (h *Handlers) enqueue(typ string, payload any, opts ...asynq.Option) {
	b, err := json.Marshal(payload)
	if err != nil {
		h.m.emails.WithLabelValues("enqueue_failed").Inc()
		h.log.Error("email task payload", "task", typ, "err", err)
		return
	}
	opts = append(opts, asynq.Queue(QueueDefault), asynq.MaxRetry(emailMaxRetry), asynq.Timeout(emailTimeout))
	_, err = h.mail.Queue.Enqueue(asynq.NewTask(typ, b), opts...)
	if err != nil && !errors.Is(err, asynq.ErrDuplicateTask) && !errors.Is(err, asynq.ErrTaskIDConflict) {
		h.m.emails.WithLabelValues("enqueue_failed").Inc()
		h.log.Error("enqueue email task", "task", typ, "err", err)
	}
}

// decode reads a task payload. A payload that does not parse never will, so no retry.
func decode(t *asynq.Task, v any) error {
	if err := json.Unmarshal(t.Payload(), v); err != nil {
		return fmt.Errorf("%w: bad %s payload: %v", asynq.SkipRetry, t.Type(), err)
	}
	return nil
}

func (h *Handlers) emailNotification(ctx context.Context, t *asynq.Task) error {
	var p notificationPayload
	if err := decode(t, &p); err != nil {
		return err
	}
	n, ok, err := h.mail.Svc.ClaimNotificationEmail(ctx, p.ID)
	if err != nil {
		return err
	}
	if !ok {
		h.m.emails.WithLabelValues("skipped").Inc()
		return nil
	}
	msg, err := h.mail.Renderer.Notification(n)
	if err != nil {
		h.m.emails.WithLabelValues("failed").Inc()
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err) // missing copy is a code bug; resending cannot fix it
	}
	return h.deliver(ctx, msg, func(ctx context.Context) error { return h.mail.Svc.ReleaseNotificationEmail(ctx, p.ID) })
}

func (h *Handlers) emailDigest(ctx context.Context, t *asynq.Task) error {
	var p digestPayload
	if err := decode(t, &p); err != nil {
		return err
	}
	d, ok, err := h.mail.Svc.ClaimDigestEmail(ctx, p.UserID, p.PactID, p.Kind)
	if err != nil {
		return err
	}
	if !ok {
		h.m.emails.WithLabelValues("skipped").Inc()
		return nil
	}
	msg, err := h.mail.Renderer.Digest(d)
	if err != nil {
		h.m.emails.WithLabelValues("failed").Inc()
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	return h.deliver(ctx, msg, func(ctx context.Context) error { return h.mail.Svc.ReleaseDigestEmail(ctx, d.IDs) })
}

func (h *Handlers) emailInvite(ctx context.Context, t *asynq.Task) error {
	var p invitePayload
	if err := decode(t, &p); err != nil {
		return err
	}
	inv, ok, err := h.mail.Svc.ClaimInviteEmail(ctx, p.Token)
	if err != nil {
		return err
	}
	if !ok {
		h.m.emails.WithLabelValues("skipped").Inc()
		return nil
	}
	msg, err := h.mail.Renderer.Invite(inv)
	if err != nil {
		h.m.emails.WithLabelValues("failed").Inc()
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	return h.deliver(ctx, msg, func(ctx context.Context) error { return h.mail.Svc.ReleaseInviteEmail(ctx, p.Token) })
}

// deliver sends a claimed message. On failure it gives the claim back, so the asynq retry
// finds something to send; without that a retry would see "already emailed" and drop it.
func (h *Handlers) deliver(ctx context.Context, msg notify.Message, release func(context.Context) error) error {
	if err := h.mail.Sender.Send(ctx, msg); err != nil {
		h.m.emails.WithLabelValues("failed").Inc()
		// The task context may be the thing that expired, so release on a fresh one.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if rerr := release(rctx); rerr != nil {
			h.log.Error("release email claim", "err", rerr)
		}
		return fmt.Errorf("send email: %w", err)
	}
	h.m.emails.WithLabelValues("sent").Inc()
	return nil
}
