package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// Service bundles dependencies shared by every use-case.
type Service struct {
	st    *store.Store
	clock domain.Clock
}

func New(st *store.Store, clock domain.Clock) *Service {
	return &Service{st: st, clock: clock}
}

func newID() uuid.UUID { return uuid.Must(uuid.NewV7()) }

// Line colours from the design direction contract: one fixed colour per member.
const (
	lineColorBacker = "#0F766E"
	lineColorDoer   = "#9D174D"
)

// Notification is the outbox payload consumed by the notify worker (PLAN 3.2).
type Notification struct {
	UserID    uuid.UUID  `json:"user_id"`
	Kind      string     `json:"kind"`
	PactID    uuid.UUID  `json:"pact_id"`
	CheckInID *uuid.UUID `json:"check_in_id,omitempty"`
}

const topicNotify = "notify"

// enqueueNotification writes to the outbox inside the caller's transaction, so a
// notification exists if and only if the change that caused it committed.
func enqueueNotification(ctx context.Context, q *store.Queries, n Notification) error {
	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}
	if err := q.InsertOutbox(ctx, store.InsertOutboxParams{Topic: topicNotify, Payload: payload}); err != nil {
		return fmt.Errorf("outbox: %w", err)
	}
	return nil
}

func loadTerms(p store.Pact) (domain.Terms, error) {
	t, err := domain.ParseTerms(p.Terms)
	if err != nil {
		return domain.Terms{}, fmt.Errorf("pact %s has unreadable terms: %w", p.ID, err)
	}
	return t, nil
}
