package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// RelayResult reports one outbox pass. Notifications are the rows now visible in the
// app, so the caller can enqueue emails for them after the transaction committed
// (ARCHITECTURE §3: never enqueue inside a transaction that might roll back).
type RelayResult struct {
	Notifications []Notification
	Skipped       int // rows the relay could not understand; dispatched so they never block the queue
}

// RelayOutbox turns pending outbox rows into in-app notifications. Fetching, inserting and
// marking dispatched share one transaction, and FOR UPDATE SKIP LOCKED lets several worker
// replicas relay at once without delivering an event twice.
func (s *Service) RelayOutbox(ctx context.Context, batch int32) (RelayResult, error) {
	var res RelayResult
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		res = RelayResult{}
		rows, err := q.FetchPendingOutbox(ctx, batch)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			n, ok := parseNotify(row)
			if !ok {
				res.Skipped++
				continue
			}
			if err := q.InsertNotification(ctx, store.InsertNotificationParams{UserID: n.UserID, Kind: n.Kind, Payload: row.Payload}); err != nil {
				return fmt.Errorf("notification for outbox %d: %w", row.ID, err)
			}
			res.Notifications = append(res.Notifications, n)
		}
		if len(ids) == 0 {
			return nil
		}
		return q.MarkOutboxDispatched(ctx, ids)
	})
	if err != nil {
		return RelayResult{}, err
	}
	return res, nil
}

func parseNotify(row store.Outbox) (Notification, bool) {
	if row.Topic != topicNotify {
		return Notification{}, false
	}
	var n Notification
	if err := json.Unmarshal(row.Payload, &n); err != nil {
		return Notification{}, false
	}
	if n.UserID == uuid.Nil || n.PactID == uuid.Nil || n.Kind == "" {
		return Notification{}, false
	}
	return n, true
}
