//go:build integration

package service

import (
	"testing"
	"time"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func (f *fixture) outboxKinds() map[string]int {
	f.t.Helper()
	rows, err := f.st.Pool.Query(f.ctx, `select payload->>'kind', count(*) from outbox where dispatched_at is null group by 1`)
	f.must(err)
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		f.must(rows.Scan(&kind, &n))
		out[kind] = n
	}
	f.must(rows.Err())
	return out
}

func (f *fixture) drainOutbox() {
	f.t.Helper()
	_, err := f.svc.RelayOutbox(f.ctx, 1000)
	f.must(err)
}

func (f *fixture) remind() int {
	f.t.Helper()
	n, err := f.svc.SendReminders(f.ctx, 500)
	f.must(err)
	return n
}

// SPEC §9: 3 h and 30 min before the cutoff, while the check-in is still open, the
// member is reminded. Cutoff on 2 Nov is 23:59 WIB.
func TestSendRemindersBeforeCutoff(t *testing.T) {
	f := newFixture(t)
	f.activePactWith(nil)
	f.drainOutbox()

	f.clock.Set(at(2, 20, 0)) // 3 h 59 min before the cutoff: too early
	if n := f.remind(); n != 0 {
		t.Fatalf("early reminders = %d", n)
	}

	f.clock.Set(at(2, 21, 0)) // 2 h 59 min before: the 3 h reminder is due, for both members
	if n := f.remind(); n != 2 {
		t.Fatalf("3h reminders = %d, want 2 (one per member)", n)
	}
	if got := f.outboxKinds()["reminder_cutoff_3h"]; got != 2 {
		t.Fatalf("outbox = %v", f.outboxKinds())
	}
	f.drainOutbox()
	if n := f.remind(); n != 0 {
		t.Fatalf("the job runs every 5 minutes; a repeat sent %d duplicates", n)
	}

	f.clock.Set(at(2, 23, 35)) // 24 min before: only the 30 min reminder is new
	if n := f.remind(); n != 2 {
		t.Fatalf("30m reminders = %d, want 2", n)
	}
	if got := f.outboxKinds(); got["reminder_cutoff_30m"] != 2 || got["reminder_cutoff_3h"] != 0 {
		t.Fatalf("outbox = %v", got)
	}
	if n := f.remind(); n != 0 {
		t.Fatalf("repeat 30m sent %d", n)
	}

	f.clock.Set(at(3, 0, 5)) // cutoff passed: no reminder for what is already late
	if n := f.remind(); n != 0 {
		t.Fatalf("after cutoff sent %d", n)
	}
}

// Both reminder windows can be open the first time the job sees a check-in (it was
// down, or the pact was created late). The member gets the urgent one only.
func TestSendRemindersLateFirstSightSendsOnlyTheUrgentOne(t *testing.T) {
	f := newFixture(t)
	f.activePactWith(nil)
	f.drainOutbox()
	f.clock.Set(at(2, 23, 40))
	if n := f.remind(); n != 2 {
		t.Fatalf("reminders = %d, want 2", n)
	}
	if got := f.outboxKinds(); got["reminder_cutoff_30m"] != 2 || got["reminder_cutoff_3h"] != 0 {
		t.Fatalf("outbox = %v", got)
	}
}

func TestSendRemindersSkipsCheckInsThatNeedNothing(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	f.drainOutbox()
	doer := f.checkIn(p.ID, f.doer.ID, 2)

	f.clock.Set(at(2, 10, 0))
	_, err := f.svc.SubmitProof(f.ctx, f.doer.ID, doer.ID, proof(30))
	f.must(err)
	_, err = f.svc.DeclareRest(f.ctx, f.backer.ID, f.checkIn(p.ID, f.backer.ID, 2).ID)
	f.must(err)
	f.drainOutbox()

	f.clock.Set(at(2, 21, 0))
	if n := f.remind(); n != 0 {
		t.Fatalf("submitted and rest check-ins were reminded: %d (%v)", n, f.outboxKinds())
	}
}

// SPEC §9: the reviewer hears about a proof whose review deadline is 2 h away.
func TestSendRemindersToReviewerBeforeReviewDeadline(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	doer := f.checkIn(p.ID, f.doer.ID, 2)
	f.clock.Set(at(2, 10, 0))
	submitted, err := f.svc.SubmitProof(f.ctx, f.doer.ID, doer.ID, proof(30))
	f.must(err)
	f.drainOutbox()
	if submitted.ReviewDeadline == nil {
		t.Fatal("submitted check-in has no review deadline")
	}

	// The next day's cutoff reminders fall in the same window, so count by kind.
	f.clock.Set(submitted.ReviewDeadline.Add(-3 * time.Hour))
	f.remind()
	if got := f.outboxKinds()["review_deadline_soon"]; got != 0 {
		t.Fatalf("3 h before the review deadline sent %d", got)
	}
	f.clock.Set(submitted.ReviewDeadline.Add(-90 * time.Minute))
	f.remind()
	if got := f.outboxKinds()["review_deadline_soon"]; got != 1 {
		t.Fatalf("review reminders = %d, want 1", got)
	}
	f.drainOutbox()
	var to store.Notification
	rows := f.notifications(f.backer.ID)
	for _, r := range rows {
		if r.Kind == "review_deadline_soon" {
			to = r
		}
	}
	if to.Kind == "" {
		t.Fatalf("backer (the reviewer) got no review_deadline_soon: %+v", rows)
	}
	f.remind()
	if got := f.outboxKinds()["review_deadline_soon"]; got != 0 {
		t.Fatalf("a repeat sent another review reminder")
	}
}
