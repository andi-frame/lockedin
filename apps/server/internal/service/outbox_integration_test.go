//go:build integration

package service

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func (f *fixture) pendingOutbox() int {
	f.t.Helper()
	var n int
	f.must(f.st.Pool.QueryRow(f.ctx, `select count(*) from outbox where dispatched_at is null`).Scan(&n))
	return n
}

func (f *fixture) notifications(user uuid.UUID) []store.Notification {
	f.t.Helper()
	rows, _, _, err := f.svc.ListNotifications(f.ctx, user, nil, 100, false)
	f.must(err)
	return rows
}

func TestRelayOutboxTurnsEventsIntoNotifications(t *testing.T) {
	f := newFixture(t)
	p := f.scheduledPact() // joining, signing and scheduling each write outbox rows
	pending := f.pendingOutbox()
	if pending < 2 {
		t.Fatalf("setup: expected pending outbox rows, got %d", pending)
	}

	res, err := f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	if len(res.Notifications) != pending || res.Skipped != 0 {
		t.Fatalf("relay = %d notifications, %d skipped, want %d", len(res.Notifications), res.Skipped, pending)
	}
	if f.pendingOutbox() != 0 {
		t.Fatal("outbox rows must be marked dispatched in the same transaction")
	}
	total := 0
	for _, u := range []uuid.UUID{f.backer.ID, f.doer.ID} {
		rows := f.notifications(u)
		total += len(rows)
		scheduled := 0
		for _, r := range rows {
			var n Notification
			f.must(json.Unmarshal(r.Payload, &n))
			if n.PactID != p.ID || n.UserID != u || n.Kind != r.Kind {
				t.Fatalf("payload = %+v for row %+v", n, r)
			}
			if r.Kind == "pact_scheduled" {
				scheduled++
			}
		}
		if scheduled != 1 {
			t.Fatalf("user %s got %d pact_scheduled notifications", u, scheduled)
		}
	}
	if total != pending {
		t.Fatalf("%d outbox rows became %d notifications", pending, total)
	}

	// Running again is a no-op: nothing is delivered twice.
	res, err = f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	if len(res.Notifications) != 0 {
		t.Fatalf("second relay delivered %d", len(res.Notifications))
	}
	if got := len(f.notifications(f.doer.ID)) + len(f.notifications(f.backer.ID)); got != pending {
		t.Fatalf("%d notifications after a second relay, want %d", got, pending)
	}
}

func TestRelayOutboxRespectsBatchSize(t *testing.T) {
	f := newFixture(t)
	f.scheduledPact()
	pending := f.pendingOutbox()
	res, err := f.svc.RelayOutbox(f.ctx, 1)
	f.must(err)
	if len(res.Notifications) != 1 || f.pendingOutbox() != pending-1 {
		t.Fatalf("batch of 1 delivered %d, %d pending of %d", len(res.Notifications), f.pendingOutbox(), pending)
	}
}

// A row the relay cannot understand must not block the queue forever: it is marked
// dispatched and reported as skipped so the job can log it.
func TestRelayOutboxSkipsUnknownTopicsAndBrokenPayloads(t *testing.T) {
	f := newFixture(t)
	f.must(f.st.InsertOutbox(f.ctx, store.InsertOutboxParams{Topic: "billing", Payload: []byte(`{}`)}))
	f.must(f.st.InsertOutbox(f.ctx, store.InsertOutboxParams{Topic: topicNotify, Payload: []byte(`{"kind":"day_missed"}`)}))
	f.must(f.st.InsertOutbox(f.ctx, store.InsertOutboxParams{Topic: topicNotify, Payload: []byte(`[1]`)}))

	res, err := f.svc.RelayOutbox(f.ctx, 100)
	f.must(err)
	if len(res.Notifications) != 0 || res.Skipped != 3 {
		t.Fatalf("relay = %d notifications, %d skipped", len(res.Notifications), res.Skipped)
	}
	if f.pendingOutbox() != 0 {
		t.Fatal("skipped rows must not stay pending")
	}
}

// Several relays at once (two worker replicas) must deliver each event exactly once.
func TestRelayOutboxConcurrentRelaysDeliverOnce(t *testing.T) {
	f := newFixture(t)
	f.scheduledPact()
	pending := f.pendingOutbox()
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := f.svc.RelayOutbox(f.ctx, 1); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if _, err := f.svc.RelayOutbox(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	if got := len(f.notifications(f.doer.ID)) + len(f.notifications(f.backer.ID)); got != pending {
		t.Fatalf("%d events produced %d notifications", pending, got)
	}
}
