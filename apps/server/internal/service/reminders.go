package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// Reminder kinds and windows, SPEC §9. The 3 h window stops 30 min before the cutoff, so a
// check-in first seen late gets the urgent reminder only, not two in a row.
const (
	reminderCutoff3h  = "reminder_cutoff_3h"
	reminderCutoff30m = "reminder_cutoff_30m"
	reminderReview    = "review_deadline_soon"

	reminderUrgent   = 30 * time.Minute
	reminderEarly    = 3 * time.Hour
	reminderReview2h = 2 * time.Hour
)

// SendReminders writes the time-based notifications into the outbox: the member is reminded
// 3 h and 30 min before the cutoff while the check-in is still open, and the reviewer 2 h before
// a review deadline. It runs every few minutes, so each reminder is recorded in reminders_sent
// in the same transaction as its outbox row; a repeat or a second worker sends nothing.
func (s *Service) SendReminders(ctx context.Context, batch int32) (int, error) {
	now := s.clock.Now()
	sent := 0
	for _, w := range []struct {
		kind     string
		from, to time.Time
	}{
		{reminderCutoff3h, now.Add(reminderUrgent), now.Add(reminderEarly)},
		{reminderCutoff30m, now, now.Add(reminderUrgent)},
	} {
		rows, err := s.st.ListCutoffReminderCandidates(ctx, store.ListCutoffReminderCandidatesParams{FromAt: w.from, ToAt: w.to, Kind: w.kind, MaxRows: batch})
		if err != nil {
			return sent, err
		}
		for _, ci := range rows {
			ok, err := s.remindOnce(ctx, ci, w.kind, "open", ci.MemberID)
			if err != nil {
				return sent, err
			}
			if ok {
				sent++
			}
		}
	}
	rows, err := s.st.ListReviewReminderCandidates(ctx, store.ListReviewReminderCandidatesParams{FromAt: now, ToAt: now.Add(reminderReview2h), MaxRows: batch})
	if err != nil {
		return sent, err
	}
	for _, ci := range rows {
		ok, err := s.remindOnce(ctx, ci, reminderReview, "submitted", ci.ReviewerID)
		if err != nil {
			return sent, err
		}
		if ok {
			sent++
		}
	}
	return sent, nil
}

// remindOnce records the reminder and queues the notification, unless it was already sent or
// the check-in moved on since it was listed (the member submitted, say).
func (s *Service) remindOnce(ctx context.Context, ci store.CheckIn, kind, wantStatus string, to uuid.UUID) (bool, error) {
	sent := false
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		cur, err := q.GetCheckIn(ctx, ci.ID)
		if err != nil {
			return err
		}
		if cur.Status != wantStatus {
			return nil
		}
		n, err := q.InsertReminderSent(ctx, store.InsertReminderSentParams{CheckInID: ci.ID, Kind: kind})
		if err != nil || n == 0 {
			return err
		}
		sent = true
		id := ci.ID
		return enqueueNotification(ctx, q, Notification{UserID: to, Kind: kind, PactID: ci.PactID, CheckInID: &id})
	})
	if err != nil {
		return false, fmt.Errorf("remind %s for check-in %s: %w", kind, ci.ID, err)
	}
	return sent, nil
}
