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
	Notifications []DeliveredNotification
	Invites       []InviteMail // plaintext tokens, to be mailed now and never stored again
	Skipped       int          // rows the relay could not understand; dispatched so they never block the queue
}

// DeliveredNotification is a notification row the relay just inserted.
type DeliveredNotification struct {
	ID int64
	Notification
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
			switch row.Topic {
			case topicNotify:
				n, ok := parseNotify(row)
				if !ok {
					res.Skipped++
					continue
				}
				id, err := q.InsertNotification(ctx, store.InsertNotificationParams{UserID: n.UserID, Kind: n.Kind, Payload: row.Payload})
				if err != nil {
					return fmt.Errorf("notification for outbox %d: %w", row.ID, err)
				}
				res.Notifications = append(res.Notifications, DeliveredNotification{ID: id, Notification: n})
			case topicInviteMail:
				m, ok := parseInviteMail(row)
				if !ok {
					res.Skipped++
					continue
				}
				res.Invites = append(res.Invites, m)
			default:
				res.Skipped++
			}
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

func parseInviteMail(row store.Outbox) (InviteMail, bool) {
	var m InviteMail
	if err := json.Unmarshal(row.Payload, &m); err != nil {
		return InviteMail{}, false
	}
	if m.PactID == uuid.Nil || m.Email == "" || m.Token == "" {
		return InviteMail{}, false
	}
	return m, true
}

func parseNotify(row store.Outbox) (Notification, bool) {
	var n Notification
	if err := json.Unmarshal(row.Payload, &n); err != nil {
		return Notification{}, false
	}
	if n.UserID == uuid.Nil || n.PactID == uuid.Nil || n.Kind == "" {
		return Notification{}, false
	}
	return n, true
}
