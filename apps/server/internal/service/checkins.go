package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

// ErrLostRace means another transaction moved the check-in first; callers may retry.
var ErrLostRace = &domain.Error{Code: "checkin.conflict", Msg: "the check-in changed while you were acting; reload and try again"}

// ProofInput is a submission or edit. Text and word count are derived server-side.
type ProofInput struct {
	BodyDoc       json.RawMessage
	Links         []string
	AttachmentIDs []uuid.UUID // attachments listed in the tray (inline ones are added automatically)
}

// SubmitProof stores a new proof version and submits (or edits) the check-in.
func (s *Service) SubmitProof(ctx context.Context, user, checkInID uuid.UUID, in ProofInput) (store.CheckIn, error) {
	parsed, err := domain.ParseProofDoc(in.BodyDoc)
	if err != nil {
		return store.CheckIn{}, err
	}
	if err := domain.ValidateLinks(in.Links); err != nil {
		return store.CheckIn{}, err
	}
	ids := uniqueIDs(append(append([]uuid.UUID{}, in.AttachmentIDs...), parsed.AttachmentIDs...))
	linksJSON, _ := json.Marshal(nonNil(in.Links))

	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventSubmit, ActorID: user},
		func(q *store.Queries, ci store.CheckIn, terms domain.Terms) (bool, error) {
			ready, pending, err := countAttachments(ctx, q, ids, user, ci.PactID)
			if err != nil {
				return false, err
			}
			if ready+pending != len(ids) {
				return false, ErrNotFound // an attachment is not the user's, or not in this pact
			}
			rules := terms.Members[ci.MemberID].Evidence
			return rules.Check(parsed.WordCount, ready, pending) == nil, nil
		},
		func(q *store.Queries, ci store.CheckIn) error {
			proof, err := q.InsertProof(ctx, store.InsertProofParams{
				ID: newID(), CheckInID: ci.ID, BodyDoc: in.BodyDoc, BodyText: parsed.Text,
				WordCount: int32(parsed.WordCount), Links: linksJSON,
			})
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return nil
			}
			_, err = q.AttachToProof(ctx, store.AttachToProofParams{ProofID: &proof.ID, Ids: ids, OwnerID: user, PactID: ci.PactID})
			return err
		})
}

func (s *Service) DeclareRest(ctx context.Context, user, checkInID uuid.UUID) (store.CheckIn, error) {
	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventDeclareRest, ActorID: user}, nil, nil)
}

func (s *Service) Approve(ctx context.Context, user, checkInID uuid.UUID) (store.CheckIn, error) {
	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventApprove, ActorID: user}, nil, nil)
}

func (s *Service) Reject(ctx context.Context, user, checkInID uuid.UUID, reason string) (store.CheckIn, error) {
	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventReject, ActorID: user, Reason: reason}, nil, nil)
}

// Override is the backer's power over an auto-approval (ADR-0007).
func (s *Service) Override(ctx context.Context, user, checkInID uuid.UUID, reason string) (store.CheckIn, error) {
	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventOverride, ActorID: user, Reason: reason}, nil, nil)
}

func (s *Service) Dispute(ctx context.Context, user, checkInID uuid.UUID, reason string) (store.CheckIn, error) {
	return s.act(ctx, user, checkInID, domain.Event{Kind: domain.EventDispute, ActorID: user, Reason: reason}, nil, nil)
}

func (s *Service) ResolveDispute(ctx context.Context, user, checkInID uuid.UUID, uphold bool, reason string) (store.CheckIn, error) {
	kind := domain.EventDismiss
	if uphold {
		kind = domain.EventUphold
	}
	return s.act(ctx, user, checkInID, domain.Event{Kind: kind, ActorID: user, Reason: reason}, nil, nil)
}

// act runs one user-initiated transition in a single transaction:
// membership → lock pact → lock check-in → domain.Transition → guarded update → effects.
// evidence (optional) decides EvidenceOK; before (optional) runs after the transition
// is known to be valid but before the state is written (e.g. inserting the proof).
func (s *Service) act(
	ctx context.Context, user, checkInID uuid.UUID, ev domain.Event,
	evidence func(*store.Queries, store.CheckIn, domain.Terms) (bool, error),
	before func(*store.Queries, store.CheckIn) error,
) (store.CheckIn, error) {
	var out store.CheckIn
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		head, err := q.GetCheckInForMember(ctx, store.GetCheckInForMemberParams{UserID: user, ID: checkInID})
		if store.IsNoRows(err) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		pact, err := q.GetPactForUpdate(ctx, head.PactID)
		if err != nil {
			return err
		}
		if pact.Status != "active" {
			return ErrPactState
		}
		ci, err := q.GetCheckInForUpdate(ctx, checkInID)
		if err != nil {
			return err
		}
		terms, err := loadTerms(pact)
		if err != nil {
			return err
		}
		evidenceOK := true
		if evidence != nil {
			if evidenceOK, err = evidence(q, ci, terms); err != nil {
				return err
			}
		}
		next, effects, err := s.transition(ctx, q, pact, terms, ci, ev, evidenceOK)
		if err != nil {
			return err
		}
		if before != nil {
			if err := before(q, ci); err != nil {
				return err
			}
		}
		if err := s.persist(ctx, q, pact, terms, ci, next, effects); err != nil {
			return err
		}
		out, err = q.GetCheckIn(ctx, checkInID)
		return err
	})
	return out, err
}

// transition builds the domain context from the locked rows and runs the state machine.
func (s *Service) transition(ctx context.Context, q *store.Queries, pact store.Pact, terms domain.Terms,
	ci store.CheckIn, ev domain.Event, evidenceOK bool) (domain.CheckIn, []domain.Effect, error) {
	member, err := q.GetPactMember(ctx, store.GetPactMemberParams{PactID: pact.ID, UserID: ci.MemberID})
	if err != nil {
		return domain.CheckIn{}, nil, err
	}
	tc := domain.TransitionCtx{
		Terms: terms, BackerID: pact.BackerID, OverridesUsed: int(pact.OverridesUsed),
		RestDaysUsed: int(member.RestDaysUsed), EvidenceOK: evidenceOK,
	}
	return domain.Transition(toDomain(ci, terms), ev, s.clock.Now(), tc)
}

// persist writes the new state (guarded by the old one) and applies every effect.
// The caller holds the pact row lock, so ledger clamps see a stable balance.
func (s *Service) persist(ctx context.Context, q *store.Queries, pact store.Pact, terms domain.Terms,
	old store.CheckIn, next domain.CheckIn, effects []domain.Effect) error {
	if len(effects) == 0 {
		return nil
	}
	n, err := q.UpdateCheckInState(ctx, store.UpdateCheckInStateParams{
		ID: old.ID, ExpectedStatus: old.Status, ExpectedFinal: old.IsFinal,
		Status: string(next.Status), IsFinal: next.IsFinal,
		SubmittedAt: next.SubmittedAt, ReviewDeadline: next.ReviewDeadline, DecidedAt: next.DecidedAt,
		DisputeDeadline: next.DisputeDeadline, DisputedAt: next.DisputedAt,
		ResolutionDeadline: next.ResolutionDeadline, OverrideDeadline: next.OverrideDeadline,
		PenaltyApplied: next.PenaltyApplied,
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLostRace
	}
	cid := old.ID
	for _, e := range effects {
		switch e.Kind {
		case domain.EffectDecision:
			reason := nilIfBlank(e.Reason)
			if err := q.InsertDecision(ctx, store.InsertDecisionParams{CheckInID: cid, ActorID: e.ActorID, Action: string(e.Action), Reason: reason}); err != nil {
				return fmt.Errorf("decision: %w", err)
			}
		case domain.EffectNotify:
			if err := enqueueNotification(ctx, q, Notification{UserID: e.To, Kind: string(e.Notify), PactID: pact.ID, CheckInID: &cid}); err != nil {
				return err
			}
		case domain.EffectPenalty:
			if err := s.applyPenalty(ctx, q, pact, terms, old, e); err != nil {
				return err
			}
		case domain.EffectReversal:
			if err := s.applyReversal(ctx, q, pact, old); err != nil {
				return err
			}
		case domain.EffectIncOverrides:
			if err := q.IncrementOverridesUsed(ctx, pact.ID); err != nil {
				return err
			}
		case domain.EffectIncRestDays:
			if err := q.IncrementRestDaysUsed(ctx, store.IncrementRestDaysUsedParams{PactID: pact.ID, UserID: old.MemberID}); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unhandled effect %q", e.Kind)
		}
	}
	return nil
}

func (s *Service) applyPenalty(ctx context.Context, q *store.Queries, pact store.Pact, terms domain.Terms, ci store.CheckIn, e domain.Effect) error {
	balance, err := q.PotBalance(ctx, pact.ID)
	if err != nil {
		return err
	}
	role := domain.RoleDoer
	if e.Ledger == domain.LedgerBackerMiss {
		role = domain.RoleBacker
	}
	draft := domain.PenaltyEntry(terms, role, e.Penalty, balance)
	var note *string
	if draft.Clamped {
		n := "clamped"
		note = &n
	}
	cid := ci.ID
	_, err = q.InsertLedgerEntry(ctx, store.InsertLedgerEntryParams{
		PactID: pact.ID, Kind: string(draft.Kind), Amount: draft.Amount, CheckInID: &cid,
		IdempotencyKey: domain.PenaltyKey(ci.ID), Note: note,
	})
	if store.IsNoRows(err) {
		return nil // already charged: idempotency key hit (SPEC §6 L3)
	}
	return err
}

func (s *Service) applyReversal(ctx context.Context, q *store.Queries, pact store.Pact, ci store.CheckIn) error {
	orig, err := q.GetLedgerEntryByKey(ctx, domain.PenaltyKey(ci.ID))
	if store.IsNoRows(err) {
		return nil // nothing was charged
	}
	if err != nil {
		return err
	}
	cid := ci.ID
	_, err = q.InsertLedgerEntry(ctx, store.InsertLedgerEntryParams{
		PactID: pact.ID, Kind: string(domain.LedgerReversal), Amount: -orig.Amount, CheckInID: &cid,
		ReversesEntryID: &orig.ID, IdempotencyKey: domain.ReversalKey(ci.ID),
	})
	if store.IsNoRows(err) {
		return nil
	}
	return err
}

// SweepDeadlines applies every passed deadline (SPEC §7 steps 1-6), one check-in
// per transaction. Safe to run concurrently: a check-in locked by another worker is
// skipped, and the guarded update plus idempotency keys prevent double charges.
func (s *Service) SweepDeadlines(ctx context.Context, batch int32) (int, error) {
	ids, err := s.st.ListDueCheckInIDs(ctx, store.ListDueCheckInIDsParams{Now: s.clock.Now(), MaxRows: batch})
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, id := range ids {
		changed, err := s.tickOne(ctx, id)
		if err != nil && !errors.Is(err, ErrLostRace) {
			return moved, fmt.Errorf("sweep %s: %w", id, err)
		}
		if changed {
			moved++
		}
	}
	return moved, nil
}

func (s *Service) tickOne(ctx context.Context, id uuid.UUID) (bool, error) {
	changed := false
	err := s.st.WithTx(ctx, func(q *store.Queries) error {
		head, err := q.GetCheckIn(ctx, id)
		if err != nil {
			return err
		}
		pact, err := q.GetPactForUpdate(ctx, head.PactID)
		if err != nil {
			return err
		}
		ci, err := q.GetCheckInForUpdateSkipLocked(ctx, id)
		if store.IsNoRows(err) {
			return nil // another worker has it
		}
		if err != nil {
			return err
		}
		terms, err := loadTerms(pact)
		if err != nil {
			return err
		}
		next, effects, err := s.transition(ctx, q, pact, terms, ci, domain.Event{Kind: domain.EventTick}, true)
		if err != nil {
			return err
		}
		changed = len(effects) > 0
		return s.persist(ctx, q, pact, terms, ci, next, effects)
	})
	return changed, err
}

// ClosePacts settles active pacts whose last day passed and whose every check-in is
// final (SPEC §7 step 7): status → settling, payout ledger row, payout record.
func (s *Service) ClosePacts(ctx context.Context, batch int32) ([]uuid.UUID, error) {
	ids, err := s.st.ListSettlablePacts(ctx, store.ListSettlablePactsParams{Now: s.clock.Now(), MaxRows: batch})
	if err != nil {
		return nil, err
	}
	var closed []uuid.UUID
	for _, id := range ids {
		err := s.st.WithTx(ctx, func(q *store.Queries) error {
			p, err := q.GetPactForUpdate(ctx, id)
			if err != nil {
				return err
			}
			open, err := q.CountNonFinalCheckIns(ctx, id)
			if err != nil {
				return err
			}
			if p.Status != "active" || open > 0 {
				return nil
			}
			if n, err := q.SetPactStatus(ctx, store.SetPactStatusParams{ID: id, FromStatus: "active", ToStatus: "settling"}); err != nil || n == 0 {
				return err
			}
			balance, err := q.PotBalance(ctx, id)
			if err != nil {
				return err
			}
			if _, err := q.InsertLedgerEntry(ctx, store.InsertLedgerEntryParams{
				PactID: id, Kind: string(domain.LedgerPayout), Amount: -balance, IdempotencyKey: domain.PayoutKey(id),
			}); err != nil && !store.IsNoRows(err) {
				return err
			}
			if err := q.CreatePayout(ctx, store.CreatePayoutParams{PactID: id, Amount: balance}); err != nil {
				return err
			}
			closed = append(closed, id)
			return s.notifyOtherMember(ctx, q, p, uuid.Nil, "pact_settled")
		})
		if err != nil {
			return closed, fmt.Errorf("close %s: %w", id, err)
		}
	}
	return closed, nil
}

func toDomain(ci store.CheckIn, terms domain.Terms) domain.CheckIn {
	return domain.CheckIn{
		ID: ci.ID, MemberID: ci.MemberID, ReviewerID: ci.ReviewerID, MemberRole: terms.Members[ci.MemberID].Role,
		Status: domain.Status(ci.Status), IsFinal: ci.IsFinal, CutoffAt: ci.CutoffAt, SubmitDeadline: ci.SubmitDeadline,
		SubmittedAt: ci.SubmittedAt, ReviewDeadline: ci.ReviewDeadline, DecidedAt: ci.DecidedAt,
		DisputeDeadline: ci.DisputeDeadline, DisputedAt: ci.DisputedAt, ResolutionDeadline: ci.ResolutionDeadline,
		OverrideDeadline: ci.OverrideDeadline, PenaltyApplied: ci.PenaltyApplied,
	}
}

func countAttachments(ctx context.Context, q *store.Queries, ids []uuid.UUID, owner, pactID uuid.UUID) (ready, pending int, err error) {
	if len(ids) == 0 {
		return 0, 0, nil
	}
	rows, err := q.CountAttachmentsByStatus(ctx, store.CountAttachmentsByStatusParams{Ids: ids, OwnerID: owner, PactID: pactID})
	if err != nil {
		return 0, 0, err
	}
	// Anything not ready (still processing, or rejected) blocks submission (SPEC §5).
	for _, r := range rows {
		if r.Status == "ready" {
			ready += int(r.N)
		} else {
			pending += int(r.N)
		}
	}
	return ready, pending, nil
}

func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
