//go:build integration

package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/testdb"
)

func TestMediaQueueEnqueuesOncePerAttachmentAndReportsBusy(t *testing.T) {
	rdb := testdb.RedisIn(t, testRedisDB)
	opt := redisOpt(rdb)
	q := NewMediaQueue(opt, 2)
	defer q.Close()
	ctx := context.Background()

	if busy, err := q.Busy(ctx); err != nil || busy {
		t.Fatalf("an empty (never created) queue: busy=%v err=%v", busy, err)
	}
	a, b := uuid.New(), uuid.New()
	if err := q.Enqueue(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := q.Enqueue(ctx, a); err != nil {
		t.Fatalf("a second enqueue of the same attachment must succeed: %v", err)
	}
	insp := asynq.NewInspector(opt)
	defer insp.Close()
	info, err := insp.GetQueueInfo(QueueMedia)
	if err != nil || info.Pending != 1 {
		t.Fatalf("pending = %d, err %v: the same attachment must not be queued twice", info.Pending, err)
	}
	if busy, _ := q.Busy(ctx); busy {
		t.Fatal("busy with 1 of 2")
	}
	if err := q.Enqueue(ctx, b); err != nil {
		t.Fatal(err)
	}
	if busy, err := q.Busy(ctx); err != nil || !busy {
		t.Fatalf("busy=%v err=%v at the limit", busy, err)
	}
}

// lockedUploads is a fakeUploads that is safe to call from the worker's goroutines.
type lockedUploads struct {
	mu sync.Mutex
	fakeUploads
}

func (l *lockedUploads) ProcessAttachment(ctx context.Context, id uuid.UUID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fakeUploads.ProcessAttachment(ctx, id)
}
func (l *lockedUploads) GiveUpAttachment(ctx context.Context, id uuid.UUID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fakeUploads.GiveUpAttachment(ctx, id)
}
func (l *lockedUploads) CollectUploads(ctx context.Context) (service.UploadSweep, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.fakeUploads.CollectUploads(ctx)
}
func (l *lockedUploads) seen() (processed, gaveUp int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.processed), len(l.gaveUp)
}

func startWorker(t *testing.T, o Options) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, o) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run returned %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("worker did not stop")
		}
	})
}

func TestWorkerRunsMediaTasksOnTheirOwnServer(t *testing.T) {
	rdb := testdb.RedisIn(t, testRedisDB)
	opt := redisOpt(rdb)
	up := &lockedUploads{}
	startWorker(t, Options{
		Redis: opt, Svc: &fakeSettlement{sweeps: []int{0}}, Log: quietLog(), Concurrency: 2, Uploads: up,
		Schedule: []Periodic{{"@every 1s", TypeOutboxRelay, QueueDefault, 10 * time.Second}},
	})

	q := NewMediaQueue(opt, 100)
	defer q.Close()
	if err := q.Enqueue(context.Background(), uuid.New()); err != nil {
		t.Fatal(err)
	}
	eventually(t, "media:process to run", 15*time.Second, func() bool { p, _ := up.seen(); return p == 1 })
}

// When a media task has used up its retries the attachment is rejected, not left "processing".
func TestWorkerRejectsTheAttachmentWhenRetriesRunOut(t *testing.T) {
	rdb := testdb.RedisIn(t, testRedisDB)
	opt := redisOpt(rdb)
	up := &lockedUploads{fakeUploads: fakeUploads{err: errors.New("ffmpeg crashed")}}
	startWorker(t, Options{
		Redis: opt, Svc: &fakeSettlement{sweeps: []int{0}}, Log: quietLog(), Concurrency: 2, Uploads: up,
		Schedule: []Periodic{{"@every 1s", TypeOutboxRelay, QueueDefault, 10 * time.Second}},
	})

	client := asynq.NewClient(opt)
	defer client.Close()
	id := uuid.New()
	payload, _ := json.Marshal(mediaPayload{AttachmentID: id})
	// No retries left from the start, so the first failure is the last.
	if _, err := client.Enqueue(asynq.NewTask(TypeMediaProcess, payload), asynq.Queue(QueueMedia), asynq.MaxRetry(0)); err != nil {
		t.Fatal(err)
	}
	eventually(t, "the attachment to be given up", 15*time.Second, func() bool { _, g := up.seen(); return g == 1 })
}
