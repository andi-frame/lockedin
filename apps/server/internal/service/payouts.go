package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// MarkPayoutPaid records that the backer settled the IOU (SPEC §3). Only the backer
// can, only while the pact is settling, and repeating it changes nothing.
func (s *Service) MarkPayoutPaid(ctx context.Context, user, pactID uuid.UUID, note *string) (store.Payout, error) {
	var out store.Payout
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		p, err := s.memberPactForUpdate(ctx, q, user, pactID)
		if err != nil {
			return err
		}
		if p.BackerID != user {
			return ErrNotBacker
		}
		if p.Status != "settling" {
			return ErrPactState
		}
		first, err := q.MarkPayoutPaid(ctx, store.MarkPayoutPaidParams{PactID: pactID, MarkedPaidNote: note})
		if err != nil {
			return err
		}
		if first == 1 {
			if err := s.notifyOtherMember(ctx, q, p, user, "payout_marked_paid"); err != nil {
				return err
			}
		}
		out, err = q.GetPayout(ctx, pactID)
		return err
	})
	return out, err
}

// ConfirmPayout is the doer acknowledging receipt. Their confirmation alone completes
// the pact (SPEC §3), whether or not the backer marked it paid first. Confirming a
// completed pact again is a no-op that returns the payout.
func (s *Service) ConfirmPayout(ctx context.Context, user, pactID uuid.UUID) (store.Payout, error) {
	var out store.Payout
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		p, err := s.memberPactForUpdate(ctx, q, user, pactID)
		if err != nil {
			return err
		}
		m, err := q.GetPactMember(ctx, store.GetPactMemberParams{PactID: pactID, UserID: user})
		if err != nil {
			return err
		}
		if m.Role != "doer" {
			return ErrNotDoer
		}
		switch p.Status {
		case "completed":
			out, err = q.GetPayout(ctx, pactID)
			return err
		case "settling":
		default:
			return ErrPactState
		}
		if _, err := q.ConfirmPayout(ctx, pactID); err != nil {
			return err
		}
		if n, err := q.SetPactStatus(ctx, store.SetPactStatusParams{ID: pactID, FromStatus: "settling", ToStatus: "completed"}); err != nil || n == 0 {
			if err != nil {
				return err
			}
			return ErrPactState
		}
		if err := s.notifyOtherMember(ctx, q, p, user, "payout_confirmed"); err != nil {
			return err
		}
		out, err = q.GetPayout(ctx, pactID)
		return err
	})
	return out, err
}
