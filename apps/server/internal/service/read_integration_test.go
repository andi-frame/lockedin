//go:build integration

package service

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func hasAll(t *testing.T, who string, got []Action, want ...Action) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("%s actions = %v, want %q among them", who, got, w)
		}
	}
}

func hasNone(t *testing.T, who string, got []Action, banned ...Action) {
	t.Helper()
	for _, b := range banned {
		if slices.Contains(got, b) {
			t.Errorf("%s actions = %v, must not offer %q", who, got, b)
		}
	}
}

func TestPactViewIsMembershipFiltered(t *testing.T) {
	f := newFixture(t)
	stranger := f.user("eka@tepati.test", "Eka")

	draft, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: "Draf", Terms: workedTerms(f.backer.ID)})
	f.must(err)
	v, err := f.svc.GetPactView(f.ctx, f.backer.ID, draft.ID)
	f.must(err)
	if v.MyRole != domain.RoleBacker || len(v.Members) != 1 || v.Members[0].Accepted || v.Balance != 0 || v.Payout != nil {
		t.Fatalf("draft view = %+v", v)
	}

	p := f.scheduledPact()
	bv, err := f.svc.GetPactView(f.ctx, f.backer.ID, p.ID)
	f.must(err)
	dv, err := f.svc.GetPactView(f.ctx, f.doer.ID, p.ID)
	f.must(err)
	if bv.MyRole != domain.RoleBacker || dv.MyRole != domain.RoleDoer {
		t.Errorf("roles = %s / %s", bv.MyRole, dv.MyRole)
	}
	if bv.Balance != 1000 || bv.Pact.Status != "scheduled" || len(bv.Members) != 2 {
		t.Errorf("scheduled view = %+v", bv)
	}
	for _, m := range bv.Members {
		if !m.Accepted || m.DisplayName == "" || m.LineColor == "" {
			t.Errorf("member = %+v, want accepted with a name and a line colour", m)
		}
	}
	if bv.Members[0].Role != domain.RoleBacker {
		t.Errorf("the backer should come first, got %s", bv.Members[0].Role)
	}
	if _, err := f.svc.GetPactView(f.ctx, stranger.ID, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
}

func TestListPactViewsPaginatesNewestFirst(t *testing.T) {
	f := newFixture(t)
	stranger := f.user("eka@tepati.test", "Eka")
	for _, title := range []string{"A", "B", "C"} {
		_, err := f.svc.CreateDraft(f.ctx, f.backer.ID, DraftInput{Title: title, Terms: workedTerms(f.backer.ID)})
		f.must(err)
	}
	_, err := f.svc.CreateDraft(f.ctx, stranger.ID, DraftInput{Title: "Milik Eka", Terms: workedTerms(stranger.ID)})
	f.must(err)

	page1, next, err := f.svc.ListPactViews(f.ctx, f.backer.ID, nil, 2)
	f.must(err)
	if len(page1) != 2 || next == nil || page1[0].Pact.Title != "C" || page1[1].Pact.Title != "B" {
		t.Fatalf("page 1 = %v, next %v", titles(page1), next)
	}
	page2, next, err := f.svc.ListPactViews(f.ctx, f.backer.ID, next, 2)
	f.must(err)
	if len(page2) != 1 || next != nil || page2[0].Pact.Title != "A" {
		t.Fatalf("page 2 = %v, next %v", titles(page2), next)
	}
}

func titles(vs []PactView) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Pact.Title)
	}
	return out
}

func TestCheckInViewActionsFollowTheStateMachine(t *testing.T) {
	f := newFixture(t)
	stranger := f.user("eka@tepati.test", "Eka")
	p := f.activePactWith(nil)
	mon := f.checkIn(p.ID, f.doer.ID, 2)

	view := func(user uuid.UUID) CheckInView {
		t.Helper()
		v, err := f.svc.GetCheckInView(f.ctx, user, mon.ID)
		f.must(err)
		return v
	}

	// Open: the doer can submit or rest, and nobody can review yet.
	d, b := view(f.doer.ID), view(f.backer.ID)
	hasAll(t, "doer (open)", d.Actions, ActionSubmit, ActionDeclareRest)
	hasNone(t, "doer (open)", d.Actions, ActionApprove, ActionReject, ActionOverride)
	if len(b.Actions) != 0 {
		t.Errorf("backer (open) actions = %v, want none", b.Actions)
	}
	if d.Member.UserID != f.doer.ID || d.Reviewer.UserID != f.backer.ID || d.PactTitle != "UTBK" {
		t.Errorf("member/reviewer/title = %v / %v / %q", d.Member.UserID, d.Reviewer.UserID, d.PactTitle)
	}
	if d.OverridesRemaining != nil || b.OverridesRemaining == nil || *b.OverridesRemaining != 3 {
		t.Errorf("overrides remaining: doer %v, backer %v (want nil and 3)", d.OverridesRemaining, b.OverridesRemaining)
	}
	if d.Proof != nil || len(d.Versions) != 0 || len(d.Decisions) != 0 {
		t.Errorf("nothing submitted yet, got proof %v, %d versions, %d decisions", d.Proof, len(d.Versions), len(d.Decisions))
	}

	// Submitted: the doer may edit, the backer may approve or reject.
	f.clock.Set(at(2, 10, 0))
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, mon.ID, proof(30))
	f.must(err)
	d, b = view(f.doer.ID), view(f.backer.ID)
	hasAll(t, "doer (submitted)", d.Actions, ActionEditProof)
	hasNone(t, "doer (submitted)", d.Actions, ActionApprove, ActionSubmit)
	hasAll(t, "backer (submitted)", b.Actions, ActionApprove, ActionReject)
	hasNone(t, "backer (submitted)", b.Actions, ActionOverride)
	if d.Proof == nil || d.Proof.Version != 1 || len(d.Versions) != 1 || d.Proof.WordCount != 30 {
		t.Errorf("latest proof = %+v, versions %d", d.Proof, len(d.Versions))
	}

	// Rejected: the doer may dispute, and the decision shows up for both members.
	f.clock.Set(at(2, 11, 0))
	_, err = f.svc.Reject(f.ctx, f.backer.ID, mon.ID, "buktinya belum cukup")
	f.must(err)
	d, b = view(f.doer.ID), view(f.backer.ID)
	hasAll(t, "doer (rejected)", d.Actions, ActionDispute)
	hasNone(t, "backer (rejected)", b.Actions, ActionApprove, ActionReject, ActionResolveDispute)
	if n := len(b.Decisions); n != 2 || b.Decisions[0].Action != "submit" || b.Decisions[1].Action != "reject" || b.Decisions[1].IsPower {
		t.Errorf("decisions should be the submission then the rejection, oldest first: %+v", b.Decisions)
	}

	// Disputed: only the backer can resolve. Resolving is a power action and is flagged as one.
	_, err = f.svc.Dispute(f.ctx, f.doer.ID, mon.ID, "saya sudah kirim semua soal")
	f.must(err)
	d, b = view(f.doer.ID), view(f.backer.ID)
	hasAll(t, "backer (disputed)", b.Actions, ActionResolveDispute)
	hasNone(t, "doer (disputed)", d.Actions, ActionResolveDispute)
	_, err = f.svc.ResolveDispute(f.ctx, f.backer.ID, mon.ID, true, "setelah dilihat lagi, cukup")
	f.must(err)
	last := view(f.backer.ID).Decisions
	if got := last[len(last)-1]; got.Action != "uphold" || !got.IsPower {
		t.Errorf("last decision = %+v, want an uphold flagged as power", got)
	}

	if _, err := f.svc.GetCheckInView(f.ctx, stranger.ID, mon.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
}

func TestCheckInViewOffersOverrideAfterAutoApproval(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	tue := f.checkIn(p.ID, f.doer.ID, 3)
	f.clock.Set(at(3, 10, 0))
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, tue.ID, proof(30))
	f.must(err)
	f.clock.Set(at(5, 0, 1)) // past the review deadline (cutoff 23:59 on the 3rd + 24 h)
	f.sweep()

	v, err := f.svc.GetCheckInView(f.ctx, f.backer.ID, tue.ID)
	f.must(err)
	if v.CheckIn.Status != "auto_approved" {
		t.Fatalf("status = %s", v.CheckIn.Status)
	}
	hasAll(t, "backer (auto-approved)", v.Actions, ActionOverride)
	if v.OverridesRemaining == nil || *v.OverridesRemaining != 3 {
		t.Errorf("remaining = %v", v.OverridesRemaining)
	}
	_, err = f.svc.Override(f.ctx, f.backer.ID, tue.ID, "ternyata soalnya dikerjakan asal")
	f.must(err)
	v, err = f.svc.GetCheckInView(f.ctx, f.backer.ID, tue.ID)
	f.must(err)
	if v.OverridesRemaining == nil || *v.OverridesRemaining != 2 || len(v.Actions) != 0 {
		t.Errorf("after an override: remaining %v, actions %v", v.OverridesRemaining, v.Actions)
	}
	dv, _ := f.svc.GetCheckInView(f.ctx, f.doer.ID, tue.ID)
	hasNone(t, "doer (overridden)", dv.Actions, ActionDispute) // the doer cannot dispute an override
}

func TestNoActionsOnAPactThatIsNotActive(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact() // scheduled, not active yet
	mon := f.checkIn(p.ID, f.doer.ID, 2)
	v, err := f.svc.GetCheckInView(f.ctx, f.doer.ID, mon.ID)
	f.must(err)
	if len(v.Actions) != 0 {
		t.Errorf("actions on a scheduled pact = %v, want none", v.Actions)
	}
}

func TestListPactCheckIns(t *testing.T) {
	f := newFixture(t)
	stranger := f.user("eka@tepati.test", "Eka")
	p := f.activePactWith(nil)

	all, err := f.svc.ListPactCheckIns(f.ctx, f.doer.ID, p.ID, nil, nil)
	f.must(err)
	if len(all) != 24+20 {
		t.Fatalf("a full pact has 24 doer + 20 backer check-ins, got %d", len(all))
	}
	from, to := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	week, err := f.svc.ListPactCheckIns(f.ctx, f.backer.ID, p.ID, &from, &to)
	f.must(err)
	if len(week) != 6+5 {
		t.Fatalf("week 1 has Mon-Sat for the doer and Mon-Fri for the backer, got %d", len(week))
	}
	if _, err := f.svc.ListPactCheckIns(f.ctx, stranger.ID, p.ID, nil, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
}

func TestLedgerPageKeepsTheRunningBalanceAcrossPages(t *testing.T) {
	f := newFixture(t)
	stranger := f.user("eka@tepati.test", "Eka")
	p := f.settlingPact() // +1000 initial, -50 doer miss, +50 backer miss, -1000 payout

	page1, err := f.svc.LedgerPage(f.ctx, f.doer.ID, p.ID, nil, 2)
	f.must(err)
	if len(page1.Lines) != 2 || page1.Next == nil || page1.Balance != 0 {
		t.Fatalf("page 1 = %d lines, next %v, balance %d", len(page1.Lines), page1.Next, page1.Balance)
	}
	if page1.Lines[0].Kind != "payout" || page1.Lines[0].BalanceAfter != 0 || page1.Lines[0].Amount != -1000 {
		t.Errorf("newest line = %+v", page1.Lines[0])
	}
	page2, err := f.svc.LedgerPage(f.ctx, f.doer.ID, p.ID, page1.Next, 10)
	f.must(err)
	if len(page2.Lines) != 2 || page2.Next != nil || page2.Balance != 0 {
		t.Fatalf("page 2 = %d lines, next %v", len(page2.Lines), page2.Next)
	}
	if first := page2.Lines[1]; first.Kind != "pot_initial" || first.BalanceAfter != 1000 {
		t.Errorf("oldest line = %+v", first)
	}
	for _, l := range append(page1.Lines, page2.Lines...) {
		if l.Kind == "doer_miss" || l.Kind == "backer_miss" {
			if l.CheckInID == nil || !l.CheckInLocalDate.Valid || l.CheckInMemberID == nil {
				t.Errorf("a penalty line must say which day and member it is about: %+v", l)
			}
		}
	}
	if _, err := f.svc.LedgerPage(f.ctx, stranger.ID, p.ID, nil, 10); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
}

func TestReviewQueueIsSortedAndPaginated(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	f.clock.Set(at(2, 10, 0))
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, f.checkIn(p.ID, f.doer.ID, 2).ID, proof(30))
	f.must(err)
	f.clock.Set(at(3, 10, 0))
	_, err = f.svc.SubmitProof(f.ctx, f.doer.ID, f.checkIn(p.ID, f.doer.ID, 3).ID, proof(45))
	f.must(err)

	page1, next, err := f.svc.ReviewQueue(f.ctx, f.backer.ID, nil, 1)
	f.must(err)
	if len(page1) != 1 || next == nil {
		t.Fatalf("page 1 = %d items, next %v", len(page1), next)
	}
	first := page1[0]
	if first.CheckIn.LocalDate.Day() != 2 || first.WordCount != 30 || first.Member.UserID != f.doer.ID || first.PactTitle != "UTBK" {
		t.Errorf("first item = %+v", first)
	}
	page2, next, err := f.svc.ReviewQueue(f.ctx, f.backer.ID, next, 1)
	f.must(err)
	if len(page2) != 1 || next != nil || page2[0].CheckIn.LocalDate.Day() != 3 || page2[0].WordCount != 45 {
		t.Fatalf("page 2 = %+v, next %v", page2, next)
	}
	if mine, _, err := f.svc.ReviewQueue(f.ctx, f.doer.ID, nil, 10); err != nil || len(mine) != 0 {
		t.Errorf("the doer reviews nothing here, got %d items (%v)", len(mine), err)
	}
}

func TestTodayAggregatesMyDayAndMyPacts(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	f.clock.Set(at(2, 9, 0))

	d, err := f.svc.Today(f.ctx, f.doer.ID)
	f.must(err)
	if !d.ServerTime.Equal(at(2, 9, 0)) || len(d.CheckIns) != 1 || d.CheckIns[0].CheckIn.LocalDate.Day() != 2 || d.CheckIns[0].PactTitle != "UTBK" {
		t.Fatalf("doer today = %+v", d)
	}
	if len(d.Pacts) != 1 {
		t.Fatalf("pacts = %d", len(d.Pacts))
	}
	tp := d.Pacts[0]
	if tp.Pact.ID != p.ID || tp.MyRole != domain.RoleDoer || tp.Partner.UserID != f.backer.ID || tp.Balance != 1000 {
		t.Errorf("today pact = %+v", tp)
	}
	if tp.NextDeadline == nil || !tp.NextDeadline.Equal(at(3, 0, 29)) { // 23:59 cutoff + 30 min grace
		t.Errorf("next deadline = %v", tp.NextDeadline)
	}
	if len(tp.Recent) != 1 || tp.Recent[0].Kind != "pot_initial" {
		t.Errorf("recent ledger = %+v", tp.Recent)
	}

	b, err := f.svc.Today(f.ctx, f.backer.ID)
	f.must(err)
	if b.ReviewCount != 0 || len(b.CheckIns) != 1 || b.CheckIns[0].CheckIn.MemberID != f.backer.ID {
		t.Fatalf("backer today = %+v", b)
	}
	_, err = f.svc.SubmitProof(f.ctx, f.doer.ID, f.checkIn(p.ID, f.doer.ID, 2).ID, proof(30))
	f.must(err)
	b, _ = f.svc.Today(f.ctx, f.backer.ID)
	if b.ReviewCount != 1 {
		t.Errorf("review count = %d, want 1", b.ReviewCount)
	}
	d, _ = f.svc.Today(f.ctx, f.doer.ID)
	if d.CheckIns[0].WordCount == nil || *d.CheckIns[0].WordCount != 30 {
		t.Errorf("word count = %v", d.CheckIns[0].WordCount)
	}

}

func TestTodayShowsYesterdayWhileItsGraceLasts(t *testing.T) {
	f := newFixture(t)
	f.activePactWith(nil)
	// Just after midnight the 2nd is still open for its 30 minutes of grace, so Today shows both days.
	f.clock.Set(at(3, 0, 10))
	f.sweep()
	d, err := f.svc.Today(f.ctx, f.doer.ID)
	f.must(err)
	if len(d.CheckIns) != 2 || d.CheckIns[0].CheckIn.LocalDate.Day() != 2 || d.CheckIns[1].CheckIn.LocalDate.Day() != 3 {
		t.Fatalf("today at 00:10 = %d check-ins, want the 2nd (grace) then the 3rd", len(d.CheckIns))
	}
	f.clock.Set(at(3, 0, 40)) // grace is over: the 2nd is missed and drops off
	f.sweep()
	d, _ = f.svc.Today(f.ctx, f.doer.ID)
	if len(d.CheckIns) != 1 || d.CheckIns[0].CheckIn.LocalDate.Day() != 3 {
		t.Errorf("after grace = %d check-ins, want only the 3rd", len(d.CheckIns))
	}
}

func TestNotificationsInbox(t *testing.T) {
	f := newFixture(t)
	add := func(user uuid.UUID, kind string) {
		f.must(f.st.InsertNotification(f.ctx, store.InsertNotificationParams{UserID: user, Kind: kind, Payload: []byte(`{"pact_id":"x"}`)}))
	}
	add(f.doer.ID, "proof_rejected")
	add(f.doer.ID, "day_missed")
	add(f.doer.ID, "pact_settled")
	add(f.backer.ID, "proof_submitted")

	page1, next, unread, err := f.svc.ListNotifications(f.ctx, f.doer.ID, nil, 2, false)
	f.must(err)
	if len(page1) != 2 || next == nil || unread != 3 || page1[0].Kind != "pact_settled" {
		t.Fatalf("page 1 = %d, next %v, unread %d", len(page1), next, unread)
	}
	page2, next, _, err := f.svc.ListNotifications(f.ctx, f.doer.ID, next, 2, false)
	f.must(err)
	if len(page2) != 1 || next != nil || page2[0].Kind != "proof_rejected" {
		t.Fatalf("page 2 = %+v, next %v", page2, next)
	}

	backerNote, _, _, _ := f.svc.ListNotifications(f.ctx, f.backer.ID, nil, 10, false)
	// Marking someone else's id is ignored, and marking our own works.
	f.must(f.svc.MarkNotificationsRead(f.ctx, f.doer.ID, []int64{page1[0].ID, backerNote[0].ID}))
	_, _, unread, _ = f.svc.ListNotifications(f.ctx, f.doer.ID, nil, 10, false)
	if unread != 2 {
		t.Errorf("doer unread = %d, want 2", unread)
	}
	_, _, backerUnread, _ := f.svc.ListNotifications(f.ctx, f.backer.ID, nil, 10, false)
	if backerUnread != 1 {
		t.Errorf("backer unread = %d, a doer must not be able to mark the backer's notification", backerUnread)
	}
	unreadOnly, _, _, _ := f.svc.ListNotifications(f.ctx, f.doer.ID, nil, 10, true)
	if len(unreadOnly) != 2 {
		t.Errorf("unread only = %d, want 2", len(unreadOnly))
	}
}
