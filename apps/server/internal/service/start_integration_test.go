//go:build integration

package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

// SPEC §3: nothing moves toward `scheduled` once the start date has passed in the pact's zone, so
// check-ins are never generated already overdue.
func TestStartDatePassedStopsTheFlow(t *testing.T) {
	st := testdb.New(t)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	// 2026-11-01 12:00 in Jakarta; termsFor starts the pact on 2026-11-02.
	clock := domain.NewFakeClock(time.Date(2026, 11, 1, 12, 0, 0, 0, jakarta))
	svc := service.New(st, clock)
	ctx := context.Background()
	backer, doer := newUser(t, st, "andi@tepati.test", "Andi"), newUser(t, st, "bima@tepati.test", "Bima")

	// A draft that starts in the past is refused at once.
	past := termsFor(backer.ID)
	past.StartsOn, past.EndsOn = domain.MustDate("2026-10-20"), domain.MustDate("2026-11-20")
	if _, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "Lama", Terms: past}); domain.CodeOf(err) != "pact.start_passed" {
		t.Fatalf("CreateDraft in the past: code %q (%v)", domain.CodeOf(err), err)
	}

	p, err := svc.CreateDraft(ctx, backer.ID, service.DraftInput{Title: "UTBK", Terms: termsFor(backer.ID)})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.Propose(ctx, backer.ID, p.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	joined, err := svc.JoinByInvite(ctx, doer.ID, token)
	if err != nil {
		t.Fatal(err)
	}

	// The start day itself is still fine: its cutoff is ahead. The backer signs on the 2nd...
	clock.Set(time.Date(2026, 11, 2, 12, 0, 0, 0, jakarta))
	if _, err := svc.Accept(ctx, backer.ID, p.ID, joined.TermsHash, backer.DisplayName); err != nil {
		t.Fatalf("signing on the start day: %v", err)
	}

	// ...and the doer comes back on the 3rd: too late, and nothing was scheduled.
	clock.Set(time.Date(2026, 11, 3, 8, 0, 0, 0, jakarta))
	if _, err := svc.Accept(ctx, doer.ID, p.ID, joined.TermsHash, doer.DisplayName); domain.CodeOf(err) != "pact.start_passed" {
		t.Fatalf("signing after the start day: code %q (%v)", domain.CodeOf(err), err)
	}
	if _, err := svc.Propose(ctx, backer.ID, p.ID, nil); domain.CodeOf(err) != "pact.start_passed" {
		t.Fatalf("inviting again after the start day: code %q (%v)", domain.CodeOf(err), err)
	}
	if got, _ := svc.GetPact(ctx, backer.ID, p.ID); got.Status != "proposed" {
		t.Fatalf("status = %q, want proposed (nothing scheduled)", got.Status)
	}

	// Moving the dates later is the way out, and it clears the signature as every edit does.
	later := termsFor(backer.ID)
	later.Members = map[uuid.UUID]domain.MemberTerms{backer.ID: later.Members[backer.ID], doer.ID: later.Members[uuid.Nil]}
	later.StartsOn, later.EndsOn = domain.MustDate("2026-11-09"), domain.MustDate("2026-12-06")
	if _, err := svc.UpdateTerms(ctx, backer.ID, p.ID, service.DraftInput{Title: "UTBK", Terms: later}); err != nil {
		t.Fatalf("moving the dates later: %v", err)
	}
	moved := later
	moved.StartsOn = domain.MustDate("2026-10-01") // an edit into the past is refused too
	if _, err := svc.UpdateTerms(ctx, backer.ID, p.ID, service.DraftInput{Title: "UTBK", Terms: moved}); domain.CodeOf(err) != "pact.start_passed" {
		t.Fatalf("editing into the past: code %q (%v)", domain.CodeOf(err), err)
	}
}
