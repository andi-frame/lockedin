package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

type fakeUploads struct {
	processed []uuid.UUID
	gaveUp    []uuid.UUID
	err       error
	sweep     service.UploadSweep
	sweeps    int
}

func (f *fakeUploads) ProcessAttachment(_ context.Context, id uuid.UUID) error {
	f.processed = append(f.processed, id)
	return f.err
}
func (f *fakeUploads) GiveUpAttachment(_ context.Context, id uuid.UUID) error {
	f.gaveUp = append(f.gaveUp, id)
	return nil
}
func (f *fakeUploads) CollectUploads(context.Context) (service.UploadSweep, error) {
	f.sweeps++
	return f.sweep, f.err
}

func uploadHandlers(u *fakeUploads) (*Handlers, *Metrics) {
	m := NewMetrics()
	h := NewHandlers(&fakeSettlement{sweeps: []int{0}}, slog.New(slog.NewTextHandler(io.Discard, nil)), m).WithUploads(u)
	return h, m
}

func mediaTask(id uuid.UUID) *asynq.Task {
	b, _ := json.Marshal(map[string]any{"attachment_id": id})
	return asynq.NewTask(TypeMediaProcess, b)
}

func TestMediaProcessRunsTheService(t *testing.T) {
	u := &fakeUploads{}
	h, m := uploadHandlers(u)
	id := uuid.New()
	if err := h.Mux().ProcessTask(context.Background(), mediaTask(id)); err != nil {
		t.Fatal(err)
	}
	if len(u.processed) != 1 || u.processed[0] != id {
		t.Fatalf("processed = %v", u.processed)
	}
	if got := testutil.ToFloat64(m.runs.WithLabelValues(TypeMediaProcess, "ok")); got != 1 {
		t.Errorf("ok runs = %v", got)
	}
}

func TestMediaProcessFailureIsRetried(t *testing.T) {
	u := &fakeUploads{err: errors.New("vips timed out")}
	h, _ := uploadHandlers(u)
	err := h.Mux().ProcessTask(context.Background(), mediaTask(uuid.New()))
	if err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
}

func TestMediaProcessWithABrokenPayloadIsNotRetried(t *testing.T) {
	h, _ := uploadHandlers(&fakeUploads{})
	for _, payload := range []string{"not json", `{"attachment_id":"nope"}`, `{}`} {
		err := h.Mux().ProcessTask(context.Background(), asynq.NewTask(TypeMediaProcess, []byte(payload)))
		if !errors.Is(err, asynq.SkipRetry) {
			t.Errorf("%s: err = %v, want SkipRetry", payload, err)
		}
	}
}

func TestExhaustedMediaJobsRejectTheAttachment(t *testing.T) {
	u := &fakeUploads{}
	h, _ := uploadHandlers(u)
	id := uuid.New()
	h.mediaFailed(context.Background(), mediaTask(id))
	if len(u.gaveUp) != 1 || u.gaveUp[0] != id {
		t.Fatalf("gaveUp = %v", u.gaveUp)
	}
	h.mediaFailed(context.Background(), asynq.NewTask(TypeMediaProcess, []byte("junk"))) // must not panic
	h.mediaFailed(context.Background(), asynq.NewTask(TypeOutboxRelay, nil))             // other tasks are not its business
	if len(u.gaveUp) != 1 {
		t.Fatalf("gaveUp = %v", u.gaveUp)
	}
}

func TestUploadsGCRunsTheSweepAndCountsIt(t *testing.T) {
	u := &fakeUploads{sweep: service.UploadSweep{Removed: 3, Requeued: 2}}
	h, m := uploadHandlers(u)
	if err := h.Mux().ProcessTask(context.Background(), asynq.NewTask(TypeUploadsGC, nil)); err != nil {
		t.Fatal(err)
	}
	if u.sweeps != 1 {
		t.Fatalf("sweeps = %d", u.sweeps)
	}
	if got := testutil.ToFloat64(m.uploads.WithLabelValues("removed")); got != 3 {
		t.Errorf("removed = %v", got)
	}
	if got := testutil.ToFloat64(m.uploads.WithLabelValues("requeued")); got != 2 {
		t.Errorf("requeued = %v", got)
	}
}

func TestMediaProcessIsServedOnlyWhenUploadsAreConfigured(t *testing.T) {
	h, _ := newTestHandlers(&fakeSettlement{sweeps: []int{0}})
	err := h.Mux().ProcessTask(context.Background(), mediaTask(uuid.New()))
	if err == nil {
		t.Fatal("a worker without uploads accepted a media task; it would drop it silently")
	}
}

func TestMediaProcessIsNotPeriodic(t *testing.T) {
	for _, p := range Schedule() {
		if p.Type == TypeMediaProcess {
			t.Fatal("media:process is event-driven")
		}
	}
}
