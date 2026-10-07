//go:build integration

package service

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func TestAgreementFlowSchedulesPactFundsPotAndGeneratesCheckIns(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact()

	if p.Status != "scheduled" || p.ScheduledAt == nil {
		t.Fatalf("want scheduled, got %s", p.Status)
	}
	if got := f.balance(p.ID); got != 1000 {
		t.Fatalf("pot funded with initial_pot: want 1000, got %d", got)
	}
	n, err := f.st.CountCheckInsForPact(f.ctx, p.ID)
	f.must(err)
	if n != 24+20 {
		t.Fatalf("doer Mon-Sat (24) + backer Mon-Fri (20): want 44, got %d", n)
	}
	terms, err := loadTerms(p)
	f.must(err)
	if _, ok := terms.Members[f.doer.ID]; !ok {
		t.Fatal("doer slot must be re-keyed to the doer's id")
	}

	// First doer check-in: Monday 2026-11-02 23:59 WIB = 16:59Z, reviewed by the backer.
	rows, err := f.st.ListCheckInsForPact(f.ctx, store.ListCheckInsForPactParams{
		PactID: p.ID, FromDate: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), ToDate: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
	})
	f.must(err)
	for _, ci := range rows {
		if ci.MemberID == ci.ReviewerID {
			t.Fatal("nobody reviews their own check-in")
		}
		if ci.CutoffAt.UTC().Format(time.RFC3339) != "2026-11-02T16:59:00Z" {
			t.Fatalf("cutoff: %s", ci.CutoffAt.UTC())
		}
	}
	if len(rows) != 2 {
		t.Fatalf("both members have Monday scheduled: got %d rows", len(rows))
	}
}

func TestAcceptingAStaleHashFails(t *testing.T) {
	f := newFixture(t)
	p, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "UTBK", Terms: workedTerms(f.backer.ID)})
	f.must(err)
	token, err := f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)
	f.must(err)
	stale := p.TermsHash // hash before the doer joined
	p, err = f.svc.JoinByInvite(f.ctx, f.doer.ID, token)
	f.must(err)

	if _, err := f.svc.Accept(f.ctx, f.doer.ID, p.ID, stale, "Bima"); !errors.Is(err, ErrTermsMismatch) {
		t.Fatalf("stale hash: want ErrTermsMismatch, got %v", err)
	}
	if domain.CodeOf(ErrTermsMismatch) != "pact.terms_mismatch" {
		t.Fatal("stable code")
	}

	// Backer signs, then edits: both signatures are cleared and the old hash is dead.
	_, err = f.svc.Accept(f.ctx, f.backer.ID, p.ID, p.TermsHash, "Andi")
	f.must(err)
	terms, _ := loadTerms(p)
	terms.GraceMinutes = 60
	edited, err := f.svc.UpdateTerms(f.ctx, f.backer.ID, p.ID, DraftInput{Title: p.Title, Terms: terms})
	f.must(err)
	if edited.TermsHash == p.TermsHash || edited.TermsVersion != p.TermsVersion+1 {
		t.Fatal("edit must change hash and bump version")
	}
	signed, err := f.st.CountAcceptances(f.ctx, store.CountAcceptancesParams{PactID: p.ID, TermsHash: &edited.TermsHash})
	f.must(err)
	if signed != 0 {
		t.Fatalf("edit must clear signatures, %d remain", signed)
	}
	if _, err := f.svc.Accept(f.ctx, f.doer.ID, p.ID, p.TermsHash, "Bima"); !errors.Is(err, ErrTermsMismatch) {
		t.Fatalf("old hash after edit: %v", err)
	}
}

func TestDoubleAcceptIsIdempotent(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact()
	// A repeated accept after scheduling is refused without side effects.
	if _, err := f.svc.Accept(f.ctx, f.doer.ID, p.ID, p.TermsHash, "Bima"); !errors.Is(err, ErrPactState) {
		t.Fatalf("want ErrPactState, got %v", err)
	}
	if f.balance(p.ID) != 1000 {
		t.Fatal("pot must be funded exactly once")
	}
	n, _ := f.st.CountCheckInsForPact(f.ctx, p.ID)
	if n != 44 {
		t.Fatalf("check-ins generated exactly once, got %d", n)
	}

	// Same member signing twice before the other signs does not schedule.
	g := newFixture(t)
	q, _ := g.svc.CreateDraft(g.ctx, g.backer.ID, DraftInput{Title: "x", Terms: workedTerms(g.backer.ID)})
	tok, _ := g.svc.Propose(g.ctx, g.backer.ID, q.ID, nil)
	q, _ = g.svc.JoinByInvite(g.ctx, g.doer.ID, tok)
	g.svc.Accept(g.ctx, g.backer.ID, q.ID, q.TermsHash, "Andi")
	q, err := g.svc.Accept(g.ctx, g.backer.ID, q.ID, q.TermsHash, "Andi")
	g.must(err)
	if q.Status != "proposed" {
		t.Fatalf("one member signing twice must not schedule, got %s", q.Status)
	}
}

func TestInviteRules(t *testing.T) {
	f := newFixture(t)
	p, _ := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "x", Terms: workedTerms(f.backer.ID)})
	token, _ := f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)

	if _, err := f.svc.JoinByInvite(f.ctx, f.backer.ID, token); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("backer cannot join own pact: %v", err)
	}
	if _, err := f.svc.Accept(f.ctx, f.backer.ID, p.ID, p.TermsHash, "Andi"); !errors.Is(err, ErrMemberMissing) {
		t.Fatalf("cannot sign before the doer joins: %v", err)
	}
	if _, err := f.svc.JoinByInvite(f.ctx, f.doer.ID, "not-a-token"); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("unknown token: %v", err)
	}
	f.must(func() error { _, err := f.svc.JoinByInvite(f.ctx, f.doer.ID, token); return err }())
	third := f.user("citra@tepati.test", "Citra")
	if _, err := f.svc.JoinByInvite(f.ctx, third.ID, token); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("token is single-use: %v", err)
	}

	// Expired token.
	g := newFixture(t)
	q, _ := g.svc.CreateDraft(g.ctx, g.backer.ID, DraftInput{Title: "x", Terms: workedTerms(g.backer.ID)})
	tok, _ := g.svc.Propose(g.ctx, g.backer.ID, q.ID, nil)
	g.clock.Advance(inviteTTL + time.Minute)
	if _, err := g.svc.JoinByInvite(g.ctx, g.doer.ID, tok); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expired token: %v", err)
	}
}

func TestSignatureAndMembership(t *testing.T) {
	f := newFixture(t)
	p, _ := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "x", Terms: workedTerms(f.backer.ID)})
	tok, _ := f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)
	p, _ = f.svc.JoinByInvite(f.ctx, f.doer.ID, tok)
	if _, err := f.svc.Accept(f.ctx, f.doer.ID, p.ID, p.TermsHash, "Andi"); !errors.Is(err, ErrSignatureMismatch) {
		t.Fatalf("must type own name: %v", err)
	}
	stranger := f.user("x@tepati.test", "X")
	if _, err := f.svc.GetPact(f.ctx, stranger.ID, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member gets not found: %v", err)
	}
	if _, err := f.svc.Accept(f.ctx, stranger.ID, p.ID, p.TermsHash, "X"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member accept: %v", err)
	}
	if _, err := f.svc.Propose(f.ctx, f.doer.ID, p.ID, nil); !errors.Is(err, ErrNotBacker) {
		t.Fatalf("only backer proposes: %v", err)
	}
}

func TestDraftShapeAndLimit(t *testing.T) {
	f := newFixture(t)
	bad := workedTerms(f.backer.ID)
	bad.Members[uuid.New()] = bad.Members[uuid.Nil]
	if _, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "x", Terms: bad}); !errors.Is(err, ErrTermsShape) {
		t.Fatalf("three members: %v", err)
	}
	invalid := workedTerms(f.backer.ID)
	invalid.GraceMinutes = 999
	if _, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "x", Terms: invalid}); !errors.Is(err, domain.ErrInvalidTerms) {
		t.Fatalf("invalid terms: %v", err)
	}
	for i := range maxOpenPacts {
		if _, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "p", Terms: workedTerms(f.backer.ID)}); err != nil {
			t.Fatalf("draft %d: %v", i, err)
		}
	}
	if _, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "p", Terms: workedTerms(f.backer.ID)}); !errors.Is(err, ErrPactLimit) {
		t.Fatalf("11th pact: %v", err)
	}
}

func TestActivateDuePactsUsesPactTimezone(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact()
	// 2026-11-01 16:59Z is still Sunday in Jakarta: not yet.
	f.clock.Set(time.Date(2026, 11, 1, 16, 59, 0, 0, time.UTC))
	ids, err := f.svc.ActivateDuePacts(f.ctx)
	f.must(err)
	if len(ids) != 0 {
		t.Fatal("must not activate before local start date")
	}
	// 17:00Z is Monday 00:00 WIB.
	f.clock.Set(time.Date(2026, 11, 1, 17, 0, 0, 0, time.UTC))
	ids, err = f.svc.ActivateDuePacts(f.ctx)
	f.must(err)
	if len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("want pact activated, got %v", ids)
	}
	ids, _ = f.svc.ActivateDuePacts(f.ctx)
	if len(ids) != 0 {
		t.Fatal("activation is idempotent")
	}
}

func TestOutboxGetsNotifications(t *testing.T) {
	f := newFixture(t)
	f.scheduledPact()
	rows, err := f.st.FetchPendingOutbox(f.ctx, 100)
	f.must(err)
	kinds := map[string]int{}
	for _, r := range rows {
		var n Notification
		_ = json.Unmarshal(r.Payload, &n)
		kinds[n.Kind]++
	}
	if kinds["member_joined"] != 1 || kinds["terms_signed"] != 1 || kinds["pact_scheduled"] != 2 {
		t.Fatalf("unexpected notifications: %v", kinds)
	}
}
