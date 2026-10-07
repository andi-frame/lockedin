//go:build integration

package ctl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

var wib = time.FixedZone("WIB", 7*3600)

// Thursday 5 Nov 2026, 10:00 WIB.
var now = time.Date(2026, 11, 5, 10, 0, 0, 0, wib)

func statuses(t *testing.T, st *store.Store, s Seeded) map[string]int {
	t.Helper()
	rows, err := st.ListCheckInsForPact(context.Background(), store.ListCheckInsForPactParams{
		PactID: s.Pact.ID, FromDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ToDate: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, r := range rows {
		out[r.Status]++
	}
	return out
}

// The scenario PLAN 3.1 verifies by hand: an active pact with check-ins whose deadlines are
// already past, which the worker (system clock) then settles.
func TestSeedOverdueGivesTheWorkerSomethingToSettle(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedOverdue(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Pact.Status != "active" {
		t.Fatalf("pact is %q, want active", s.Pact.Status)
	}
	before := statuses(t, st, s)
	if before["open"] == 0 || before["missed"] != 0 {
		t.Fatalf("before the sweep: %v", before)
	}

	moved, err := service.New(st, domain.NewFakeClock(now)).SweepDeadlines(ctx, 500)
	if err != nil {
		t.Fatal(err)
	}
	after := statuses(t, st, s)
	if moved != 6 || after["missed"] != 6 { // 2 Nov, 3 Nov, 4 Nov for both members
		t.Fatalf("moved %d, statuses %v; want 6 missed", moved, after)
	}
	if after["open"] == 0 {
		t.Fatalf("today's and later check-ins must stay open: %v", after)
	}
}

func TestSeedReusesItsUsersAcrossRuns(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	a, err := SeedOverdue(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SeedInvite(ctx, st, now)
	if err != nil {
		t.Fatalf("second seed run: %v", err)
	}
	if a.Backer.ID != b.Backer.ID {
		t.Fatal("the seed backer must be reused, not recreated")
	}
	if a.Pact.ID == b.Pact.ID {
		t.Fatal("each run creates its own pact")
	}
}

func TestSeedInviteLeavesAProposedPactWithAWorkingLink(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedInvite(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	if s.Pact.Status != "proposed" || s.InviteToken == "" || s.InviteEmail == "" {
		t.Fatalf("seed = %+v", s)
	}
	p, err := service.New(st, domain.NewFakeClock(now)).PreviewInvite(ctx, s.InviteToken)
	if err != nil {
		t.Fatalf("the printed invite token does not work: %v", err)
	}
	if p.ID != s.Pact.ID {
		t.Fatal("invite points at another pact")
	}
}

func TestShowDescribesPactCheckInsAndLedger(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedOverdue(ctx, st, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.New(st, domain.NewFakeClock(now)).SweepDeadlines(ctx, 500); err != nil {
		t.Fatal(err)
	}
	out, err := Show(ctx, st, s.Pact.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{s.Pact.Title, "active", "missed", "2026-11-02", "pot_initial", "balance", s.Backer.DisplayName, s.Doer.DisplayName} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}
