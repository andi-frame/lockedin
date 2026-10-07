//go:build integration

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

var wib = time.FixedZone("WIB", 7*3600)

// at returns hh:mm WIB on a November 2026 day.
func at(day, hour, minute int) time.Time { return time.Date(2026, 11, day, hour, minute, 0, 0, wib) }

func proof(words int) ProofInput {
	text := strings.TrimSpace(strings.Repeat("soal ", words))
	doc := fmt.Sprintf(`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":%q}]}]}`, text)
	return ProofInput{BodyDoc: json.RawMessage(doc)}
}

// activePactWith schedules a pact with custom terms and activates it on 2026-11-02.
func (f *fixture) activePactWith(mutate func(*domain.Terms)) store.Pact {
	f.t.Helper()
	terms := workedTerms(f.backer.ID)
	if mutate != nil {
		mutate(&terms)
	}
	p, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "UTBK", Terms: terms})
	f.must(err)
	tok, err := f.svc.Propose(f.ctx, f.backer.ID, p.ID, nil)
	f.must(err)
	p, err = f.svc.JoinByInvite(f.ctx, f.doer.ID, tok)
	f.must(err)
	_, err = f.svc.Accept(f.ctx, f.backer.ID, p.ID, p.TermsHash, "Andi")
	f.must(err)
	_, err = f.svc.Accept(f.ctx, f.doer.ID, p.ID, p.TermsHash, "Bima")
	f.must(err)
	f.clock.Set(at(2, 0, 0))
	_, err = f.svc.ActivateDuePacts(f.ctx)
	f.must(err)
	p, err = f.st.GetPact(f.ctx, p.ID)
	f.must(err)
	if p.Status != "active" {
		f.t.Fatalf("pact not active: %s", p.Status)
	}
	return p
}

func (f *fixture) checkIn(pactID, member uuid.UUID, day int) store.CheckIn {
	f.t.Helper()
	d := time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC)
	rows, err := f.st.ListCheckInsForPact(f.ctx, store.ListCheckInsForPactParams{PactID: pactID, FromDate: d, ToDate: d})
	f.must(err)
	for _, r := range rows {
		if r.MemberID == member {
			return r
		}
	}
	f.t.Fatalf("no check-in for %s on Nov %d", member, day)
	return store.CheckIn{}
}

func (f *fixture) sweep() int {
	f.t.Helper()
	n, err := f.svc.SweepDeadlines(f.ctx, 500)
	f.must(err)
	return n
}

type ledgerSummary struct {
	byKind  map[string]int
	sum     map[string]int64
	clamped int
}

func (f *fixture) ledger(pactID uuid.UUID) ledgerSummary {
	f.t.Helper()
	rows, err := f.st.ListLedgerPage(f.ctx, store.ListLedgerPageParams{PactID: pactID, MaxRows: 1000})
	f.must(err)
	s := ledgerSummary{byKind: map[string]int{}, sum: map[string]int64{}}
	for _, r := range rows {
		s.byKind[r.Kind]++
		s.sum[r.Kind] += r.Amount
		if r.Note != nil && *r.Note == "clamped" {
			s.clamped++
		}
	}
	return s
}

func TestConcurrentSweepsChargeEachMissExactlyOnce(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	f.clock.Set(at(4, 23, 59).Add(31 * time.Minute)) // Mon, Tue, Wed all missed by both members

	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.SweepDeadlines(f.ctx, 500); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent sweep: %v", err)
	}

	l := f.ledger(p.ID)
	if l.byKind["doer_miss"] != 3 || l.byKind["backer_miss"] != 3 {
		t.Fatalf("want 3 doer + 3 backer penalties, got %v", l.byKind)
	}
	if got := f.balance(p.ID); got != 1000 {
		t.Fatalf("1000 - 150 + 150: got %d", got)
	}
	if again := f.sweep(); again != 0 {
		t.Fatalf("second sweep must be a no-op, moved %d", again)
	}
	missed := f.checkIn(p.ID, f.doer.ID, 2)
	if missed.Status != "missed" || !missed.IsFinal || !missed.PenaltyApplied {
		t.Fatalf("doer Monday: %+v", missed)
	}
}

func TestPotNeverDropsBelowFloor(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(func(tr *domain.Terms) {
		tr.InitialPot, tr.PotCap, tr.BackerCommits = 100, nil, false
		b := tr.Members[f.backer.ID]
		b.Schedule, b.PenaltyPerMiss = nil, 0
		tr.Members[f.backer.ID] = b
	})
	f.clock.Set(at(4, 23, 59).Add(31 * time.Minute))
	f.sweep()
	l := f.ledger(p.ID)
	if l.byKind["doer_miss"] != 3 || l.sum["doer_miss"] != -100 || l.clamped != 1 {
		t.Fatalf("100 → 50 → 0 → 0 (clamped row kept): %+v", l)
	}
	if f.balance(p.ID) != 0 {
		t.Fatal("pot stops at the floor")
	}
}

func TestPotNeverExceedsCap(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(func(tr *domain.Terms) {
		tr.PotCap = i64(1020)
		d := tr.Members[uuid.Nil]
		d.Schedule = []int{7} // doer only on Sundays, so Mon-Tue are backer-only days
		tr.Members[uuid.Nil] = d
	})
	f.clock.Set(at(3, 23, 59).Add(31 * time.Minute))
	f.sweep()
	l := f.ledger(p.ID)
	if l.byKind["backer_miss"] != 2 || l.sum["backer_miss"] != 20 || l.clamped != 2 {
		t.Fatalf("1000 → 1020 (+20 clamped) → 1020 (+0 clamped): %+v", l)
	}
	if f.balance(p.ID) != 1020 {
		t.Fatal("pot stops at the cap")
	}
}

func TestOverrideLimitAndPower(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(func(tr *domain.Terms) { tr.MaxOverrides = 1 })
	mon, tue := f.checkIn(p.ID, f.doer.ID, 2), f.checkIn(p.ID, f.doer.ID, 3)

	f.clock.Set(at(2, 20, 0))
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(30))
	f.must(err)
	f.clock.Set(at(3, 20, 0))
	_, err = f.svc.SubmitProof(f.ctx, f.doer.ID, tue.ID, proof(30))
	f.must(err)

	// Backer stays silent past both review deadlines (Tue cutoff + 24h).
	f.clock.Set(at(4, 23, 59).Add(time.Minute))
	f.sweep()
	if c := f.checkIn(p.ID, f.doer.ID, 2); c.Status != "auto_approved" || c.IsFinal {
		t.Fatalf("Monday should be auto-approved and still overridable: %s final=%v", c.Status, c.IsFinal)
	}

	if _, err := f.svc.Override(f.ctx, f.doer.ID, mon.ID, "saya membatalkan sendiri"); !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("doer cannot override: %v", err)
	}
	ci, err := f.svc.Override(f.ctx, f.backer.ID, mon.ID, "foto soal tidak terbaca sama sekali")
	f.must(err)
	if ci.Status != "rejected" || !ci.IsFinal {
		t.Fatalf("override: %+v", ci)
	}
	if _, err := f.svc.Override(f.ctx, f.backer.ID, tue.ID, "foto soal tidak terbaca sama sekali"); !errors.Is(err, domain.ErrOverrideLimit) {
		t.Fatalf("second override over limit: %v", err)
	}
	if _, err := f.svc.Dispute(f.ctx, f.doer.ID, mon.ID, "saya keberatan dengan pembatalan"); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("overrides cannot be disputed: %v", err)
	}

	pact, _ := f.st.GetPact(f.ctx, p.ID)
	if pact.OverridesUsed != 1 {
		t.Fatalf("overrides_used: %d", pact.OverridesUsed)
	}
	decisions, err := f.st.ListDecisions(f.ctx, mon.ID)
	f.must(err)
	last := decisions[len(decisions)-1]
	if last.Action != "override" || last.ActorID == nil || *last.ActorID != f.backer.ID || last.Reason == nil {
		t.Fatalf("override must be logged with actor and reason: %+v", last)
	}
	if l := f.ledger(p.ID); l.byKind["doer_miss"] != 1 {
		t.Fatalf("override charges once: %v", l.byKind)
	}
}

func TestDisputeUpholdDismissAndReversal(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	mon, tue, wed := f.checkIn(p.ID, f.doer.ID, 2), f.checkIn(p.ID, f.doer.ID, 3), f.checkIn(p.ID, f.doer.ID, 4)
	for _, c := range []struct {
		ci  store.CheckIn
		day int
	}{{mon, 2}, {tue, 3}} {
		f.clock.Set(at(c.day, 20, 0))
		_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, c.ci.ID, proof(30))
		f.must(err)
		f.clock.Set(at(c.day+1, 8, 0))
		_, err = f.svc.Reject(f.ctx, f.backer.ID, c.ci.ID, "lampiran tidak sesuai komitmen")
		f.must(err)
		_, err = f.svc.Dispute(f.ctx, f.doer.ID, c.ci.ID, "lampiran kedua berisi semua soal")
		f.must(err)
	}
	_, err := f.svc.ResolveDispute(f.ctx, f.backer.ID, mon.ID, true, "")
	f.must(err)
	_, err = f.svc.ResolveDispute(f.ctx, f.backer.ID, tue.ID, false, "lampiran kedua juga bukan soal UTBK")
	f.must(err)
	l := f.ledger(p.ID)
	if l.byKind["doer_miss"] != 1 {
		t.Fatalf("upheld: no coins; dismissed: one penalty. got %v", l.byKind)
	}

	// Reversal path (SPEC §6): a penalised check-in that ends up approved gets its
	// penalty reversed. Craft that state directly to exercise the service code.
	f.clock.Set(at(4, 20, 0))
	_, err = f.svc.SubmitProof(f.ctx, f.doer.ID, wed.ID, proof(30))
	f.must(err)
	f.clock.Set(at(5, 8, 0))
	_, err = f.svc.Reject(f.ctx, f.backer.ID, wed.ID, "lampiran tidak sesuai komitmen")
	f.must(err)
	_, err = f.svc.Dispute(f.ctx, f.doer.ID, wed.ID, "lampiran kedua berisi semua soal")
	f.must(err)
	_, err = f.st.Pool.Exec(f.ctx, `update check_ins set penalty_applied = true where id = $1`, wed.ID)
	f.must(err)
	_, err = f.st.Pool.Exec(f.ctx, `insert into ledger_entries (pact_id, kind, amount, check_in_id, idempotency_key)
		values ($1, 'doer_miss', -50, $2, $3)`, p.ID, wed.ID, domain.PenaltyKey(wed.ID))
	f.must(err)
	before := f.balance(p.ID)
	_, err = f.svc.ResolveDispute(f.ctx, f.backer.ID, wed.ID, true, "")
	f.must(err)
	if got := f.balance(p.ID); got != before+50 {
		t.Fatalf("reversal must restore 50 coins: %d → %d", before, got)
	}
	rev, err := f.st.GetLedgerEntryByKey(f.ctx, domain.ReversalKey(wed.ID))
	f.must(err)
	if rev.Kind != "reversal" || rev.ReversesEntryID == nil {
		t.Fatalf("reversal row: %+v", rev)
	}
}

func TestSubmitProofEvidenceAndVersions(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(func(tr *domain.Terms) {
		d := tr.Members[uuid.Nil]
		d.Evidence = domain.Evidence{MinWords: 20}
		tr.Members[uuid.Nil] = d
	})
	mon := f.checkIn(p.ID, f.doer.ID, 2)
	f.clock.Set(at(2, 20, 0))
	if _, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(5)); !errors.Is(err, domain.ErrEvidenceInsufficient) {
		t.Fatalf("too few words: %v", err)
	}
	if _, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, ProofInput{BodyDoc: json.RawMessage(`{"type":"doc","content":[{"type":"iframe"}]}`)}); !errors.Is(err, domain.ErrInvalidProofDoc) {
		t.Fatalf("disallowed node: %v", err)
	}
	if _, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, ProofInput{BodyDoc: proof(25).BodyDoc, AttachmentIDs: []uuid.UUID{uuid.New()}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown attachment: %v", err)
	}
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(25))
	f.must(err)
	f.clock.Set(at(2, 23, 0))
	ci, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(40)) // edit before cutoff
	f.must(err)
	if ci.Status != "submitted" {
		t.Fatal("edit keeps it submitted")
	}
	latest, err := f.st.GetLatestProof(f.ctx, mon.ID)
	f.must(err)
	if latest.Version != 2 || latest.WordCount != 40 {
		t.Fatalf("edit creates version 2 with server-derived word count: v%d %d words", latest.Version, latest.WordCount)
	}
}

func TestMembershipAndPactState(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact() // scheduled, not yet active
	mon := f.checkIn(p.ID, f.doer.ID, 2)
	if _, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(30)); !errors.Is(err, ErrPactState) {
		t.Fatalf("pact not active yet: %v", err)
	}
	stranger := f.user("eka@tepati.test", "Eka")
	if _, err := f.svc.Approve(f.ctx, stranger.ID, mon.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member: %v", err)
	}
}

// TestWorkedExample replays the 4-week scenario from docs/PLAN.md end to end.
func TestWorkedExample(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)

	type plan string
	const (
		doIt     plan = "approve"
		miss     plan = "miss"
		rest     plan = "rest"
		upheld   plan = "dispute-upheld"
		override plan = "override"
	)
	doerPlan := map[int]plan{3: miss, 10: miss, 17: miss, 24: miss, 5: rest, 19: rest, 12: upheld, 28: override} // override on Saturday: Sunday has no check-ins to trip over while time jumps
	backerPlan := map[int]plan{13: miss}

	for day := 2; day <= 29; day++ {
		for _, m := range []struct {
			id, reviewer uuid.UUID
			plans        map[int]plan
			sched        map[time.Weekday]bool
		}{
			{f.doer.ID, f.backer.ID, doerPlan, map[time.Weekday]bool{1: true, 2: true, 3: true, 4: true, 5: true, 6: true}},
			{f.backer.ID, f.doer.ID, backerPlan, map[time.Weekday]bool{1: true, 2: true, 3: true, 4: true, 5: true}},
		} {
			if !m.sched[at(day, 12, 0).Weekday()] {
				continue
			}
			ci := f.checkIn(p.ID, m.id, day)
			action := m.plans[day]
			if action == "" {
				action = doIt
			}
			f.clock.Set(at(day, 19, 0))
			switch action {
			case miss:
				continue
			case rest:
				_, err := f.svc.DeclareRest(f.ctx, m.id, ci.ID)
				f.must(err)
				continue
			}
			_, err := f.svc.SubmitProof(f.ctx, m.id, ci.ID, proof(30))
			f.must(err)
			f.clock.Set(at(day, 23, 59).Add(2 * time.Hour)) // next morning 01:59 WIB
			switch action {
			case doIt:
				_, err = f.svc.Approve(f.ctx, m.reviewer, ci.ID)
				f.must(err)
			case upheld:
				_, err = f.svc.Reject(f.ctx, m.reviewer, ci.ID, "bukti tidak lengkap untuk hari ini")
				f.must(err)
				_, err = f.svc.Dispute(f.ctx, m.id, ci.ID, "bukti lengkap ada di lampiran")
				f.must(err)
				_, err = f.svc.ResolveDispute(f.ctx, m.reviewer, ci.ID, true, "")
				f.must(err)
			case override:
				f.clock.Set(at(day, 23, 59).Add(25 * time.Hour)) // past review deadline
				f.sweep()
				_, err = f.svc.Override(f.ctx, f.backer.ID, ci.ID, "foto soal hari ini tidak terbaca")
				f.must(err)
			}
		}
		f.clock.Set(at(day, 23, 59).Add(31 * time.Minute))
		f.sweep()
	}

	// Let every remaining window close, then settle.
	f.clock.Set(at(29, 23, 59).Add(10 * 24 * time.Hour))
	f.sweep()
	if got := f.balance(p.ID); got != 800 {
		l := f.ledger(p.ID)
		t.Fatalf("worked example: want 800 before payout, got %d (%v %v)", got, l.byKind, l.sum)
	}
	closed, err := f.svc.ClosePacts(f.ctx, 10)
	f.must(err)
	if len(closed) != 1 {
		t.Fatalf("pact should settle, closed %v", closed)
	}
	if f.balance(p.ID) != 0 {
		t.Fatal("payout empties the pot")
	}
	payout, err := f.st.GetPayout(f.ctx, p.ID)
	f.must(err)
	if payout.Amount != 800 {
		t.Fatalf("800 coins (≈ Rp800.000) owed to the doer, got %d", payout.Amount)
	}
	pact, _ := f.st.GetPact(f.ctx, p.ID)
	if pact.Status != "settling" {
		t.Fatalf("status: %s", pact.Status)
	}
	l := f.ledger(p.ID)
	want := map[string]int{"pot_initial": 1, "doer_miss": 5, "backer_miss": 1, "payout": 1}
	for k, v := range want {
		if l.byKind[k] != v {
			t.Fatalf("ledger rows %s: want %d, got %d (%v)", k, v, l.byKind[k], l.byKind)
		}
	}
	if again, _ := f.svc.ClosePacts(f.ctx, 10); len(again) != 0 {
		t.Fatal("closing is idempotent")
	}
}
