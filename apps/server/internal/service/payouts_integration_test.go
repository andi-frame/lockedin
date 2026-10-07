//go:build integration

package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// settlingPact runs a pact with one check-in per member to `settling`. Both members miss
// their only Monday: the doer's miss takes 50 out of the pot and the backer's miss puts
// 50 back, so the pot, and the payout, stay at the initial 1000.
func (f *fixture) settlingPact() store.Pact {
	f.t.Helper()
	p := f.activePactWith(func(t *domain.Terms) {
		t.EndsOn = domain.MustDate("2026-11-03")
		for id, m := range t.Members {
			m.Schedule = []int{1} // Monday 2026-11-02 only
			t.Members[id] = m
		}
	})
	f.clock.Set(at(4, 12, 0))
	f.sweep()
	closed, err := f.svc.ClosePacts(f.ctx, 10)
	f.must(err)
	if len(closed) != 1 || closed[0] != p.ID {
		f.t.Fatalf("pact should have closed, got %v", closed)
	}
	p, err = f.st.GetPact(f.ctx, p.ID)
	f.must(err)
	if p.Status != "settling" {
		f.t.Fatalf("status = %s, want settling", p.Status)
	}
	return p
}

func (f *fixture) payoutNotifications(kind string) []Notification {
	f.t.Helper()
	rows, err := f.st.FetchPendingOutbox(f.ctx, 1000)
	f.must(err)
	var out []Notification
	for _, r := range rows {
		var n Notification
		f.must(json.Unmarshal(r.Payload, &n))
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

func TestMarkPayoutPaid(t *testing.T) {
	f := newFixture(t)
	p := f.settlingPact()
	stranger := f.user("eka@tepati.test", "Eka")

	if _, err := f.svc.MarkPayoutPaid(f.ctx, stranger.ID, p.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
	if _, err := f.svc.MarkPayoutPaid(f.ctx, f.doer.ID, p.ID, nil); !errors.Is(err, ErrNotBacker) {
		t.Fatalf("only the backer marks paid, got %v", err)
	}

	note := "transfer lewat dana"
	po, err := f.svc.MarkPayoutPaid(f.ctx, f.backer.ID, p.ID, &note)
	f.must(err)
	if po.MarkedPaidAt == nil || po.MarkedPaidNote == nil || *po.MarkedPaidNote != note || po.ConfirmedAt != nil {
		t.Fatalf("payout after marking = %+v", po)
	}
	first := *po.MarkedPaidAt

	again, err := f.svc.MarkPayoutPaid(f.ctx, f.backer.ID, p.ID, nil)
	f.must(err)
	if !again.MarkedPaidAt.Equal(first) {
		t.Error("marking twice must not move the timestamp")
	}
	if got := f.payoutNotifications("payout_marked_paid"); len(got) != 1 || got[0].UserID != f.doer.ID {
		t.Errorf("exactly one notification to the doer expected, got %+v", got)
	}
	if now, _ := f.st.GetPact(f.ctx, p.ID); now.Status != "settling" {
		t.Errorf("marking paid alone must not complete the pact, status = %s", now.Status)
	}
}

func TestConfirmPayoutCompletesThePact(t *testing.T) {
	f := newFixture(t)
	p := f.settlingPact()
	stranger := f.user("eka@tepati.test", "Eka")

	if _, err := f.svc.ConfirmPayout(f.ctx, stranger.ID, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a non-member must get not found, got %v", err)
	}
	if _, err := f.svc.ConfirmPayout(f.ctx, f.backer.ID, p.ID); !errors.Is(err, ErrNotDoer) {
		t.Fatalf("the backer cannot confirm receipt, got %v", err)
	}

	// SPEC §3: the doer's confirmation alone is enough, even if the backer never marked it paid.
	po, err := f.svc.ConfirmPayout(f.ctx, f.doer.ID, p.ID)
	f.must(err)
	if po.ConfirmedAt == nil || po.MarkedPaidAt != nil || po.Amount != 1000 {
		t.Fatalf("payout = %+v", po)
	}
	done, err := f.st.GetPact(f.ctx, p.ID)
	f.must(err)
	if done.Status != "completed" || done.CompletedAt == nil {
		t.Fatalf("pact = %s, completed_at %v", done.Status, done.CompletedAt)
	}
	if got := f.payoutNotifications("payout_confirmed"); len(got) != 1 || got[0].UserID != f.backer.ID {
		t.Errorf("exactly one notification to the backer expected, got %+v", got)
	}

	if _, err := f.svc.ConfirmPayout(f.ctx, f.doer.ID, p.ID); err != nil {
		t.Errorf("confirming again must be a no-op, got %v", err)
	}
	if _, err := f.svc.MarkPayoutPaid(f.ctx, f.backer.ID, p.ID, nil); !errors.Is(err, ErrPactState) {
		t.Errorf("marking paid after completion must fail with pact.invalid_state, got %v", err)
	}
	if got := f.payoutNotifications("payout_confirmed"); len(got) != 1 {
		t.Errorf("repeat confirm must not notify again, got %d", len(got))
	}
}

func TestPayoutNeedsASettlingPact(t *testing.T) {
	f := newFixture(t)
	p := f.activePactWith(nil)
	if _, err := f.svc.MarkPayoutPaid(f.ctx, f.backer.ID, p.ID, nil); !errors.Is(err, ErrPactState) {
		t.Errorf("mark paid on an active pact: %v", err)
	}
	if _, err := f.svc.ConfirmPayout(f.ctx, f.doer.ID, p.ID); !errors.Is(err, ErrPactState) {
		t.Errorf("confirm on an active pact: %v", err)
	}
	if _, err := f.svc.ConfirmPayout(f.ctx, f.doer.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown pact: %v", err)
	}
}
