package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// TypeMediaProcess processes one uploaded attachment (PLAN 4.2). It is enqueued by the API
// when an upload completes, never by the schedule.
const TypeMediaProcess = "media:process"

const (
	mediaMaxRetry = 3
	mediaTimeout  = 20 * time.Minute // a 200 MB video on a small worker is slow, not stuck
	// mediaConcurrency is ARCHITECTURE §6's cap on transcodes at once. asynq has one worker pool
	// per server, so media runs on its own small server (see Run) and cannot starve settlement.
	mediaConcurrency = 2
)

// Uploads is what the media handlers need from internal/service.
type Uploads interface {
	ProcessAttachment(ctx context.Context, id uuid.UUID) error
	GiveUpAttachment(ctx context.Context, id uuid.UUID) error
	CollectUploads(ctx context.Context) (service.UploadSweep, error)
}

// WithUploads turns on media:process and gives uploads:gc something to collect.
func (h *Handlers) WithUploads(u Uploads) *Handlers {
	h.uploads = u
	return h
}

type mediaPayload struct {
	AttachmentID uuid.UUID `json:"attachment_id"`
}

func decodeMedia(t *asynq.Task) (uuid.UUID, error) {
	var p mediaPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil || p.AttachmentID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("%w: bad %s payload", asynq.SkipRetry, t.Type())
	}
	return p.AttachmentID, nil
}

func (h *Handlers) mediaProcess(ctx context.Context, t *asynq.Task) error {
	id, err := decodeMedia(t)
	if err != nil {
		return err
	}
	return h.uploads.ProcessAttachment(ctx, id)
}

// mediaFailed runs when a media task has used up its retries (see Run's ErrorHandler): the
// uploader gets a rejection to act on instead of an attachment stuck in "processing".
func (h *Handlers) mediaFailed(ctx context.Context, t *asynq.Task) {
	if t.Type() != TypeMediaProcess || h.uploads == nil {
		return
	}
	id, err := decodeMedia(t)
	if err != nil {
		return
	}
	if err := h.uploads.GiveUpAttachment(ctx, id); err != nil {
		h.log.Error("reject attachment after failed processing", "attachment", id, "err", err)
	}
}

func (h *Handlers) uploadsGC(ctx context.Context) error {
	if h.uploads == nil {
		return nil
	}
	res, err := h.uploads.CollectUploads(ctx)
	h.m.uploads.WithLabelValues("removed").Add(float64(res.Removed))
	h.m.uploads.WithLabelValues("requeued").Add(float64(res.Requeued))
	if res.Removed+res.Requeued > 0 {
		h.log.Info("uploads collected", "removed", res.Removed, "requeued", res.Requeued)
	}
	return err
}

// MediaQueue is the API's side of the media queue: it enqueues media:process and says when the
// backlog is too long to accept more uploads. It implements service.MediaQueue.
type MediaQueue struct {
	client *asynq.Client
	insp   *asynq.Inspector
	max    int
}

func NewMediaQueue(redis asynq.RedisConnOpt, max int) *MediaQueue {
	return &MediaQueue{client: asynq.NewClient(redis), insp: asynq.NewInspector(redis), max: max}
}

func (q *MediaQueue) Close() error { return errors.Join(q.client.Close(), q.insp.Close()) }

// Enqueue is idempotent: the task id is the attachment id, so a second call while the task is
// queued or running is a success, not an error.
func (q *MediaQueue) Enqueue(_ context.Context, id uuid.UUID) error {
	b, err := json.Marshal(mediaPayload{AttachmentID: id})
	if err != nil {
		return err
	}
	_, err = q.client.Enqueue(asynq.NewTask(TypeMediaProcess, b),
		asynq.Queue(QueueMedia), asynq.TaskID("media-"+id.String()),
		asynq.MaxRetry(mediaMaxRetry), asynq.Timeout(mediaTimeout), asynq.Retention(0))
	if errors.Is(err, asynq.ErrTaskIDConflict) {
		return nil
	}
	return err
}

// Busy reports whether the media backlog is at its limit.
func (q *MediaQueue) Busy(context.Context) (bool, error) {
	// GetQueueInfo errors for a queue nothing has been enqueued to yet (asynq does not wrap
	// ErrQueueNotFound there), so look the queue up in the list first.
	names, err := q.insp.Queues()
	if err != nil {
		return false, err
	}
	for _, n := range names {
		if n != QueueMedia {
			continue
		}
		info, err := q.insp.GetQueueInfo(QueueMedia)
		if err != nil {
			return false, err
		}
		return info.Pending+info.Active+info.Scheduled+info.Retry >= q.max, nil
	}
	return false, nil
}

var _ service.MediaQueue = (*MediaQueue)(nil)
