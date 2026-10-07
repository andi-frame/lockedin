package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// fakeSettlement records calls and plays back scripted results.
type fakeSettlement struct {
	sweeps   []int // moved per SweepDeadlines call, in order; the last value repeats
	sweepErr error
	calls    map[string]int
	activate []uuid.UUID
	closed   []uuid.UUID
	relay    service.RelayResult
	remind   int
	closeErr error
}

func (f *fakeSettlement) hit(name string) {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[name]++
}

func (f *fakeSettlement) SweepDeadlines(_ context.Context, _ int32) (int, error) {
	f.hit("sweep")
	if f.sweepErr != nil {
		return 0, f.sweepErr
	}
	i := f.calls["sweep"] - 1
	if i >= len(f.sweeps) {
		i = len(f.sweeps) - 1
	}
	return f.sweeps[i], nil
}
func (f *fakeSettlement) ActivateDuePacts(context.Context) ([]uuid.UUID, error) {
	f.hit("activate")
	return f.activate, nil
}
func (f *fakeSettlement) ClosePacts(context.Context, int32) ([]uuid.UUID, error) {
	f.hit("close")
	return f.closed, f.closeErr
}
func (f *fakeSettlement) RelayOutbox(context.Context, int32) (service.RelayResult, error) {
	f.hit("relay")
	return f.relay, nil
}
func (f *fakeSettlement) SendReminders(context.Context, int32) (int, error) {
	f.hit("remind")
	return f.remind, nil
}

func newTestHandlers(f *fakeSettlement) (*Handlers, *Metrics) {
	m := NewMetrics()
	return NewHandlers(f, slog.New(slog.NewTextHandler(io.Discard, nil)), m), m
}

func run(t *testing.T, h *Handlers, typ string) error {
	t.Helper()
	return h.Mux().ProcessTask(context.Background(), asynq.NewTask(typ, nil))
}

func TestScheduleMatchesPlan(t *testing.T) {
	// PLAN 3.1: the periodic tasks, their cadence and the queue each one runs on.
	want := map[string]struct {
		spec, queue string
	}{
		TypeSettlementSweep: {"@every 1m", QueueCritical},
		TypePactsActivate:   {"@every 1m", QueueCritical},
		TypePactsClose:      {"@every 5m", QueueCritical},
		TypeOutboxRelay:     {"@every 5s", QueueDefault},
		TypeUploadsGC:       {"@every 1h", QueueMedia},
		TypeRemindersCutoff: {"@every 5m", QueueDefault},
	}
	got := Schedule()
	if len(got) != len(want) {
		t.Fatalf("%d periodic tasks, want %d", len(got), len(want))
	}
	for _, p := range got {
		w, ok := want[p.Type]
		if !ok || p.Spec != w.spec || p.Queue != w.queue {
			t.Errorf("%s = %s on %s, want %+v", p.Type, p.Spec, p.Queue, w)
		}
		if p.Timeout <= 0 {
			t.Errorf("%s has no timeout", p.Type)
		}
	}
	if QueueWeights[QueueCritical] != 6 || QueueWeights[QueueDefault] != 3 || QueueWeights[QueueMedia] != 1 {
		t.Errorf("queue weights = %v, want 6/3/1", QueueWeights)
	}
}

func TestEveryPeriodicTaskHasAHandler(t *testing.T) {
	h, _ := newTestHandlers(&fakeSettlement{sweeps: []int{0}})
	for _, p := range Schedule() {
		if err := run(t, h, p.Type); err != nil {
			t.Errorf("%s: %v", p.Type, err)
		}
	}
}

// A backlog (the worker was down) is worked off in one run, not one batch per minute.
func TestSweepDrainsABacklogBatchByBatch(t *testing.T) {
	f := &fakeSettlement{sweeps: []int{sweepBatch, sweepBatch, 3}}
	h, m := newTestHandlers(f)
	if err := run(t, h, TypeSettlementSweep); err != nil {
		t.Fatal(err)
	}
	if f.calls["sweep"] != 3 {
		t.Fatalf("sweep called %d times, want 3", f.calls["sweep"])
	}
	if got := testutil.ToFloat64(m.transitions.WithLabelValues("deadline")); got != float64(2*sweepBatch+3) {
		t.Fatalf("deadline transitions = %v", got)
	}
}

func TestSweepStopsAfterBoundedPasses(t *testing.T) {
	f := &fakeSettlement{sweeps: []int{sweepBatch}} // never ends
	h, _ := newTestHandlers(f)
	if err := run(t, h, TypeSettlementSweep); err != nil {
		t.Fatal(err)
	}
	if f.calls["sweep"] != maxDrainPasses {
		t.Fatalf("sweep called %d times, want the cap %d", f.calls["sweep"], maxDrainPasses)
	}
}

func TestFailedJobIsReturnedSoAsynqRecordsIt(t *testing.T) {
	boom := errors.New("db down")
	h, m := newTestHandlers(&fakeSettlement{sweepErr: boom, closeErr: boom})
	for _, typ := range []string{TypeSettlementSweep, TypePactsClose} {
		if err := run(t, h, typ); !errors.Is(err, boom) {
			t.Errorf("%s returned %v", typ, err)
		}
	}
	if got := testutil.ToFloat64(m.runs.WithLabelValues(TypeSettlementSweep, "error")); got != 1 {
		t.Fatalf("error runs = %v", got)
	}
}

func TestTransitionsAreCountedByType(t *testing.T) {
	f := &fakeSettlement{
		sweeps:   []int{0},
		activate: []uuid.UUID{uuid.New(), uuid.New()},
		closed:   []uuid.UUID{uuid.New()},
		relay:    service.RelayResult{Notifications: make([]service.Notification, 4), Skipped: 1},
		remind:   5,
	}
	h, m := newTestHandlers(f)
	for _, typ := range []string{TypePactsActivate, TypePactsClose, TypeOutboxRelay, TypeRemindersCutoff} {
		if err := run(t, h, typ); err != nil {
			t.Fatal(err)
		}
	}
	for label, want := range map[string]float64{"activated": 2, "closed": 1} {
		if got := testutil.ToFloat64(m.transitions.WithLabelValues(label)); got != want {
			t.Errorf("transitions{%s} = %v, want %v", label, got, want)
		}
	}
	if got := testutil.ToFloat64(m.relayed); got != 4 {
		t.Errorf("relayed = %v", got)
	}
	if got := testutil.ToFloat64(m.skipped); got != 1 {
		t.Errorf("skipped = %v", got)
	}
	if got := testutil.ToFloat64(m.reminders); got != 5 {
		t.Errorf("reminders = %v", got)
	}
	if got := testutil.ToFloat64(m.runs.WithLabelValues(TypeOutboxRelay, "ok")); got != 1 {
		t.Errorf("relay ok runs = %v", got)
	}
}
