//go:build integration

package service

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// proposedWithInvite returns the plaintext token Propose handed back, after the relay ran.
func (f *fixture) proposedWithInvite(email string) (pactID uuid.UUID, token string, relay RelayResult) {
	f.t.Helper()
	p, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "UTBK <November>", Terms: workedTerms(f.backer.ID)})
	f.must(err)
	token, err = f.svc.Propose(f.ctx, f.backer.ID, p.ID, &email)
	f.must(err)
	relay, err = f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	return p.ID, token, relay
}

func TestProposeWithEmailQueuesAnInviteMail(t *testing.T) {
	f := newFixture(t)
	pactID, token, res := f.proposedWithInvite("calon@tepati.test")
	if len(res.Invites) != 1 {
		t.Fatalf("relay returned %d invites, want 1", len(res.Invites))
	}
	inv := res.Invites[0]
	if inv.PactID != pactID || inv.Email != "calon@tepati.test" || inv.Token != token {
		t.Fatalf("invite = %+v", inv)
	}
	if res.Skipped != 0 {
		t.Fatalf("skipped = %d", res.Skipped)
	}
}

func TestProposeWithoutEmailQueuesNothing(t *testing.T) {
	f := newFixture(t)
	p, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "UTBK", Terms: workedTerms(f.backer.ID)})
	f.must(err)
	_, err = f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)
	f.must(err)
	res, err := f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	if len(res.Invites) != 0 {
		t.Fatalf("a link-only invite produced %d mails", len(res.Invites))
	}
}

// The plaintext token must not outlive the relay: only its hash may stay in the database.
func TestRelayScrubsTheInviteTokenFromTheOutbox(t *testing.T) {
	f := newFixture(t)
	_, token, _ := f.proposedWithInvite("calon@tepati.test")
	var leaked int
	f.must(f.st.Pool.QueryRow(f.ctx, `select count(*) from outbox where payload::text like '%' || $1 || '%'`, token).Scan(&leaked))
	if leaked != 0 {
		t.Fatalf("token still present in %d outbox rows after dispatch", leaked)
	}
}

func TestClaimInviteEmailSendsOnce(t *testing.T) {
	f := newFixture(t)
	_, token, _ := f.proposedWithInvite("calon@tepati.test")

	mail, ok, err := f.svc.ClaimInviteEmail(f.ctx, token)
	f.must(err)
	if !ok || mail.To != "calon@tepati.test" || mail.PactTitle != "UTBK <November>" || mail.BackerName != "Andi" {
		t.Fatalf("claim = %+v ok=%v", mail, ok)
	}
	if mail.ExpiresAt.IsZero() {
		t.Fatal("the mail must say when the link stops working")
	}
	if _, ok, err = f.svc.ClaimInviteEmail(f.ctx, token); err != nil || ok {
		t.Fatalf("second claim ok=%v err=%v: a retried task would send twice", ok, err)
	}

	// A failed send gives the claim back so the retry can send.
	f.must(f.svc.ReleaseInviteEmail(f.ctx, token))
	if _, ok, err = f.svc.ClaimInviteEmail(f.ctx, token); err != nil || !ok {
		t.Fatalf("claim after release ok=%v err=%v", ok, err)
	}
}

func TestClaimInviteEmailSkipsUsedOrUnknownInvites(t *testing.T) {
	f := newFixture(t)
	if _, ok, err := f.svc.ClaimInviteEmail(f.ctx, "no-such-token"); err != nil || ok {
		t.Fatalf("unknown token ok=%v err=%v", ok, err)
	}
	_, token, _ := f.proposedWithInvite("calon@tepati.test")
	_, err := f.svc.JoinByInvite(f.ctx, f.doer.ID, token)
	f.must(err)
	if _, ok, err := f.svc.ClaimInviteEmail(f.ctx, token); err != nil || ok {
		t.Fatalf("an invite already used must not be mailed: ok=%v err=%v", ok, err)
	}
}

func TestClaimNotificationEmailSendsOnce(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact()
	res, err := f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	var scheduled DeliveredNotification
	for _, n := range res.Notifications {
		if n.Kind == "pact_scheduled" && n.UserID == f.doer.ID {
			scheduled = n
		}
	}
	if scheduled.ID == 0 {
		t.Fatalf("no pact_scheduled notification for the doer in %+v", res.Notifications)
	}

	mail, ok, err := f.svc.ClaimNotificationEmail(f.ctx, scheduled.ID)
	f.must(err)
	if !ok || mail.To != f.doer.Email || mail.ToName != "Bima" || mail.PactID != p.ID || mail.PactTitle != "UTBK November" || mail.Kind != "pact_scheduled" {
		t.Fatalf("claim = %+v ok=%v", mail, ok)
	}
	if _, ok, err = f.svc.ClaimNotificationEmail(f.ctx, scheduled.ID); err != nil || ok {
		t.Fatalf("second claim ok=%v err=%v", ok, err)
	}
	f.must(f.svc.ReleaseNotificationEmail(f.ctx, scheduled.ID))
	if _, ok, err = f.svc.ClaimNotificationEmail(f.ctx, scheduled.ID); err != nil || !ok {
		t.Fatalf("claim after release ok=%v err=%v", ok, err)
	}
	if _, ok, err = f.svc.ClaimNotificationEmail(f.ctx, 999999); err != nil || ok {
		t.Fatalf("missing notification ok=%v err=%v", ok, err)
	}
}

func TestClaimDigestGathersEveryUnsentNotification(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact()
	for range 3 {
		_, err := f.st.InsertNotification(f.ctx, insertParams(f.backer.ID, "proof_submitted", p.ID))
		f.must(err)
	}
	_, err := f.st.InsertNotification(f.ctx, insertParams(f.backer.ID, "proof_edited", p.ID)) // another kind stays out
	f.must(err)

	d, ok, err := f.svc.ClaimDigestEmail(f.ctx, f.backer.ID, p.ID, "proof_submitted")
	f.must(err)
	if !ok || d.Count != 3 || d.To != f.backer.Email || d.PactTitle != "UTBK November" || len(d.IDs) != 3 {
		t.Fatalf("digest = %+v ok=%v", d, ok)
	}
	if _, ok, err = f.svc.ClaimDigestEmail(f.ctx, f.backer.ID, p.ID, "proof_submitted"); err != nil || ok {
		t.Fatalf("a second digest with nothing new must not send: ok=%v err=%v", ok, err)
	}
	f.must(f.svc.ReleaseDigestEmail(f.ctx, d.IDs))
	if d, ok, err = f.svc.ClaimDigestEmail(f.ctx, f.backer.ID, p.ID, "proof_submitted"); err != nil || !ok || d.Count != 3 {
		t.Fatalf("after release: %+v ok=%v err=%v", d, ok, err)
	}
	if strings.TrimSpace(d.ToName) == "" {
		t.Fatal("digest needs the recipient's name")
	}
}

func insertParams(user uuid.UUID, kind string, pact uuid.UUID) store.InsertNotificationParams {
	return store.InsertNotificationParams{UserID: user, Kind: kind, Payload: []byte(`{"pact_id":"` + pact.String() + `"}`)}
}
