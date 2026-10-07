// Package jobs runs Tepati's background work on asynq (ADR-0004): the settlement sweeps, the
// outbox relay and the reminders. Handlers only call one service function each; the rules
// live in internal/service and internal/domain, so the worker and the API share them
// (AGENTS.md invariant 3). Every handler is idempotent and safe to run on several workers.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// Task types.
const (
	TypeSettlementSweep = "settlement:sweep"
	TypePactsActivate   = "pacts:activate"
	TypePactsClose      = "pacts:close"
	TypeOutboxRelay     = "outbox:relay"
	TypeUploadsGC       = "uploads:gc"
	TypeRemindersCutoff = "reminders:cutoff"
)

// Queues. Weights are the share of worker attention, ARCHITECTURE §3.
const (
	QueueCritical = "critical" // settlement
	QueueDefault  = "default"  // notifications and email
	QueueMedia    = "media"    // transcoding
)

var QueueWeights = map[string]int{QueueCritical: 6, QueueDefault: 3, QueueMedia: 1}

// Periodic is one entry of the schedule.
type Periodic struct {
	Spec    string // robfig/cron spec, "@every 1m"
	Type    string
	Queue   string
	Timeout time.Duration
}

// Schedule is PLAN 3.1's periodic task table. A missed tick costs nothing: each job
// re-derives its work from the database (SPEC §10: a transition lands within 2 minutes).
func Schedule() []Periodic {
	return []Periodic{
		{"@every 1m", TypeSettlementSweep, QueueCritical, 50 * time.Second},
		{"@every 1m", TypePactsActivate, QueueCritical, 50 * time.Second},
		{"@every 5m", TypePactsClose, QueueCritical, 2 * time.Minute},
		{"@every 5s", TypeOutboxRelay, QueueDefault, 20 * time.Second},
		{"@every 1h", TypeUploadsGC, QueueMedia, 5 * time.Minute},
		{"@every 5m", TypeRemindersCutoff, QueueDefault, 2 * time.Minute},
	}
}

// Settlement is what the handlers need from internal/service.
type Settlement interface {
	SweepDeadlines(ctx context.Context, batch int32) (int, error)
	ActivateDuePacts(ctx context.Context) ([]uuid.UUID, error)
	ClosePacts(ctx context.Context, batch int32) ([]uuid.UUID, error)
	RelayOutbox(ctx context.Context, batch int32) (service.RelayResult, error)
	SendReminders(ctx context.Context, batch int32) (int, error)
}

const (
	sweepBatch     = 200
	maxDrainPasses = 10 // bounds one run well inside its timeout; the next tick continues
	closeBatch     = 100
	relayBatch     = 200
	remindBatch    = 500
)

// Handlers binds the task types to the service.
type Handlers struct {
	svc Settlement
	log *slog.Logger
	m   *Metrics
}

func NewHandlers(svc Settlement, log *slog.Logger, m *Metrics) *Handlers {
	return &Handlers{svc: svc, log: log, m: m}
}

// Mux registers every task type, each wrapped to record its duration and outcome.
func (h *Handlers) Mux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	for typ, fn := range map[string]func(context.Context) error{
		TypeSettlementSweep: h.sweep,
		TypePactsActivate:   h.activate,
		TypePactsClose:      h.closePacts,
		TypeOutboxRelay:     h.relay,
		TypeUploadsGC:       h.uploadsGC,
		TypeRemindersCutoff: h.reminders,
	} {
		mux.HandleFunc(typ, h.instrument(typ, fn))
	}
	return mux
}

func (h *Handlers) instrument(typ string, fn func(context.Context) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, _ *asynq.Task) error {
		start := time.Now()
		err := fn(ctx)
		h.m.observe(typ, time.Since(start), err)
		if err != nil {
			h.log.Error("job failed", "task", typ, "err", err)
		}
		return err
	}
}

// sweep applies every passed deadline (SPEC §7 steps 1-6). It keeps going while batches come
// back full, so a backlog after downtime clears in one run.
func (h *Handlers) sweep(ctx context.Context) error {
	for range maxDrainPasses {
		moved, err := h.svc.SweepDeadlines(ctx, sweepBatch)
		h.m.transitions.WithLabelValues("deadline").Add(float64(moved))
		if err != nil {
			return err
		}
		if moved > 0 {
			h.log.Info("deadlines applied", "check_ins", moved)
		}
		if moved < sweepBatch {
			return nil
		}
	}
	return nil
}

func (h *Handlers) activate(ctx context.Context) error {
	ids, err := h.svc.ActivateDuePacts(ctx)
	h.m.transitions.WithLabelValues("activated").Add(float64(len(ids)))
	if len(ids) > 0 {
		h.log.Info("pacts activated", "pacts", len(ids))
	}
	return err
}

func (h *Handlers) closePacts(ctx context.Context) error {
	ids, err := h.svc.ClosePacts(ctx, closeBatch)
	h.m.transitions.WithLabelValues("closed").Add(float64(len(ids)))
	if len(ids) > 0 {
		h.log.Info("pacts settled", "pacts", len(ids))
	}
	return err
}

// relay drains the outbox into notifications. Email enqueueing for the relayed rows
// arrives with PLAN 3.2, after this transaction has committed.
func (h *Handlers) relay(ctx context.Context) error {
	res, err := h.svc.RelayOutbox(ctx, relayBatch)
	if err != nil {
		return err
	}
	h.m.relayed.Add(float64(len(res.Notifications)))
	h.m.skipped.Add(float64(res.Skipped))
	if res.Skipped > 0 {
		h.log.Warn("outbox rows skipped as unreadable", "rows", res.Skipped)
	}
	return nil
}

func (h *Handlers) reminders(ctx context.Context) error {
	n, err := h.svc.SendReminders(ctx, remindBatch)
	h.m.reminders.Add(float64(n))
	if n > 0 {
		h.log.Info("reminders queued", "reminders", n)
	}
	return err
}

// uploadsGC is scheduled now so the cadence is fixed, but has nothing to collect until the
// BlobStore and the upload endpoints exist (PLAN 4.1 and 4.2 fill it in).
func (h *Handlers) uploadsGC(context.Context) error { return nil }
