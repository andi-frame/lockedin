//go:build integration

package ctl

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

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

// The Today screen's e2e needs one signed-in doer who sees all four states at once. It goes
// through the real service with a clock, so the states are the ones the rules produce.
func TestSeedTodayGivesTheDoerEveryState(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedToday(ctx, st, now, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Pacts) != 5 {
		t.Fatalf("want 5 pacts, got %d", len(s.Pacts))
	}

	svc := service.New(st, domain.NewFakeClock(now))
	view, err := svc.Today(ctx, s.Doer.ID)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{} // pact title -> status of the doer's check-in dated today
	for _, c := range view.CheckIns {
		got[c.PactTitle] = c.CheckIn.Status
	}
	want := map[string]string{
		"Today: open": "open", "Today: rules": "open", "Today: submitted": "submitted", "Today: approved": "approved", "Today: missed": "missed",
	}
	for title, status := range want {
		if got[title] != status {
			t.Errorf("%s: doer's check-in is %q, want %q (all: %v)", title, got[title], status, got)
		}
	}
	if len(view.Pacts) != 5 {
		t.Errorf("the doer has %d live pacts, want 5", len(view.Pacts))
	}
	if view.ReviewCount != 1 {
		t.Errorf("the doer has %d proofs to review, want 1 (the backer's, in the submitted pact)", view.ReviewCount)
	}
	// The missed day is a debit in its pact's passbook; the other pots are untouched.
	for _, p := range view.Pacts {
		want := int64(1000)
		if p.Pact.Title == "Today: missed" {
			want = 950 // the doer's 50-coin penalty; this backer does not commit, so nothing is added back
		}
		if p.Balance != want {
			t.Errorf("%s: balance = %d, want %d", p.Pact.Title, p.Balance, want)
		}
	}
}

func TestSeedTodayCanRunRepeatedly(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	a, err := SeedToday(ctx, st, now, "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SeedToday(ctx, st, now, "b")
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if a.Doer.ID == b.Doer.ID || a.Doer.Email == b.Doer.Email {
		t.Fatal("each run gets its own users, so the 10-pact limit is never reached")
	}
}

// The passbook scenario: a month of printed lines, so the pact page has a second page to scroll to
// and a calendar with every kind of day.
func TestSeedPassbookPrintsAMonthOfLines(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedPassbook(ctx, st, now, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Pact.Status != "active" {
		t.Fatalf("pact is %q, want active", s.Pact.Status)
	}

	svc := service.New(st, domain.NewFakeClock(now))
	page, err := svc.LedgerPage(ctx, s.Doer.ID, s.Pact.ID, nil, 100)
	if err != nil {
		t.Fatal(err)
	}
	// Opening pot, 14 doer days that cost 50 (13 missed + 1 rejected), 18 backer misses worth 20.
	if len(page.Lines) != 1+14+18 {
		t.Fatalf("the passbook has %d lines, want 33", len(page.Lines))
	}
	var sum int64
	for _, l := range page.Lines {
		sum += l.Amount
	}
	if want := int64(1000 - 14*50 + 18*20); page.Balance != want || sum != want {
		t.Fatalf("balance %d, sum of lines %d, want %d", page.Balance, sum, want)
	}
	if page.Lines[0].BalanceAfter != page.Balance {
		t.Fatalf("the newest line says %d, the pot is %d", page.Lines[0].BalanceAfter, page.Balance)
	}

	got := map[string]int{}
	rows, err := st.ListCheckInsForPact(ctx, store.ListCheckInsForPactParams{
		PactID: s.Pact.ID, FromDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ToDate: time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		who := "backer"
		if r.MemberID == s.Doer.ID {
			who = "doer"
		}
		got[who+" "+r.Status]++
	}
	want := map[string]int{
		"doer approved": 8, "doer rest": 2, "doer missed": 13, "doer rejected": 1, "doer open": 2, // today and tomorrow are still to come
		"backer approved": 6, "backer missed": 18, "backer submitted": 1, "backer open": 1, // the backer already sent today's proof
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("%s: %d, want %d (all: %v)", k, got[k], n, got)
		}
	}
}

// Advance is the test clock: it moves time to just past the pact's next deadline and runs the real
// sweep, so the miss is printed by the same code that prints it in production.
func TestAdvancePrintsTheNextMissAsADebit(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	s, err := SeedPassbook(ctx, st, now, "p2")
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, domain.NewFakeClock(now))
	before, err := svc.LedgerPage(ctx, s.Doer.ID, s.Pact.ID, nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	adv, err := Advance(ctx, st, s.Pact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if adv.Moved != 1 {
		t.Fatalf("moved %d check-ins, want 1 (the doer's day today)", adv.Moved)
	}
	after, err := svc.LedgerPage(ctx, s.Doer.ID, s.Pact.ID, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	line := after.Lines[0]
	if line.ID == before.Lines[0].ID || line.Kind != "doer_miss" || line.Amount != -50 || after.Balance != before.Balance-50 {
		t.Fatalf("newest line %+v, balance %d -> %d; want a new doer_miss of -50", line, before.Balance, after.Balance)
	}

	if _, err := Advance(ctx, st, uuid.New()); !errors.Is(err, ErrNothingToAdvance) {
		t.Fatalf("a pact with no open check-in should say so, got %v", err)
	}
}

// Two pacts share the same 23:59 cutoff. Advancing one must not print the other's miss, or specs
// that each own a pact would spoil each other's data.
func TestAdvanceLeavesOtherPactsAlone(t *testing.T) {
	st := testdb.New(t)
	ctx := context.Background()
	a, err := SeedPassbook(ctx, st, now, "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := SeedPassbook(ctx, st, now, "b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Advance(ctx, st, a.Pact.ID); err != nil {
		t.Fatal(err)
	}
	svc := service.New(st, domain.NewFakeClock(now))
	pageA, _ := svc.LedgerPage(ctx, a.Doer.ID, a.Pact.ID, nil, 1)
	pageB, err := svc.LedgerPage(ctx, b.Doer.ID, b.Pact.ID, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if pageA.Balance != 610 || pageB.Balance != 660 {
		t.Fatalf("advanced pact %d (want 610), the other %d (want 660)", pageA.Balance, pageB.Balance)
	}
	// The next advance for b still finds today's open check-in, which proves it was not swept.
	if adv, err := Advance(ctx, st, b.Pact.ID); err != nil || adv.Moved != 1 {
		t.Fatalf("advance b: %+v, %v", adv, err)
	}
}
