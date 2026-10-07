package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Status of a check-in (SPEC §5).
type Status string

const (
	StatusOpen         Status = "open"
	StatusSubmitted    Status = "submitted"
	StatusApproved     Status = "approved"
	StatusAutoApproved Status = "auto_approved"
	StatusRejected     Status = "rejected"
	StatusDisputed     Status = "disputed"
	StatusMissed       Status = "missed"
	StatusRest         Status = "rest"
)

// CheckIn is the domain view of one scheduled day for one member.
type CheckIn struct {
	ID                 uuid.UUID
	MemberID           uuid.UUID
	ReviewerID         uuid.UUID
	MemberRole         Role
	Status             Status
	IsFinal            bool
	CutoffAt           time.Time
	SubmitDeadline     time.Time
	SubmittedAt        *time.Time
	ReviewDeadline     *time.Time
	DecidedAt          *time.Time
	DisputeDeadline    *time.Time
	DisputedAt         *time.Time
	ResolutionDeadline *time.Time
	OverrideDeadline   *time.Time
	PenaltyApplied     bool
}

type EventKind string

const (
	EventSubmit      EventKind = "submit" // also used for edits and resubmissions
	EventDeclareRest EventKind = "declare_rest"
	EventApprove     EventKind = "approve"
	EventReject      EventKind = "reject"
	EventOverride    EventKind = "override"
	EventDispute     EventKind = "dispute"
	EventUphold      EventKind = "uphold"
	EventDismiss     EventKind = "dismiss"
	EventTick        EventKind = "tick" // the settlement sweep: applies every due deadline
)

type Event struct {
	Kind    EventKind
	ActorID uuid.UUID // zero for EventTick
	Reason  string
}

// TransitionCtx is everything outside the check-in row that the rules need.
type TransitionCtx struct {
	Terms         Terms
	BackerID      uuid.UUID
	OverridesUsed int  // pacts.overrides_used
	RestDaysUsed  int  // pact_members.rest_days_used for the check-in's member
	EvidenceOK    bool // result of Evidence.Check for the proof being submitted
}

// Decision actions recorded in the decisions audit table.
type DecisionAction string

const (
	ActionSubmit         DecisionAction = "submit"
	ActionApprove        DecisionAction = "approve"
	ActionReject         DecisionAction = "reject"
	ActionAutoApprove    DecisionAction = "auto_approve"
	ActionOverride       DecisionAction = "override"
	ActionDispute        DecisionAction = "dispute"
	ActionUphold         DecisionAction = "uphold"
	ActionDismiss        DecisionAction = "dismiss"
	ActionDisputeExpired DecisionAction = "dispute_expired"
	ActionRest           DecisionAction = "rest"
	ActionMissed         DecisionAction = "missed"
	ActionFinalize       DecisionAction = "finalize"
)

type NotifyKind string

const (
	NotifyProofSubmitted    NotifyKind = "proof_submitted"
	NotifyProofEdited       NotifyKind = "proof_edited"
	NotifyProofApproved     NotifyKind = "proof_approved"
	NotifyProofRejected     NotifyKind = "proof_rejected"
	NotifyProofAutoApproved NotifyKind = "proof_auto_approved"
	NotifyProofOverridden   NotifyKind = "proof_overridden"
	NotifyRejectionFinal    NotifyKind = "rejection_final"
	NotifyRestDeclared      NotifyKind = "rest_declared"
	NotifyDayMissed         NotifyKind = "day_missed"
	NotifyDisputeOpened     NotifyKind = "dispute_opened"
	NotifyDisputeUpheld     NotifyKind = "dispute_upheld"
	NotifyDisputeDismissed  NotifyKind = "dispute_dismissed"
)

type EffectKind string

const (
	EffectDecision     EffectKind = "decision"
	EffectNotify       EffectKind = "notify"
	EffectPenalty      EffectKind = "penalty"  // service clamps and writes a ledger row
	EffectReversal     EffectKind = "reversal" // service reverses the check-in's penalty row
	EffectIncOverrides EffectKind = "inc_overrides"
	EffectIncRestDays  EffectKind = "inc_rest_days"
)

// Effect is a side effect the service must apply in the same transaction.
type Effect struct {
	Kind    EffectKind
	Action  DecisionAction // decision
	ActorID *uuid.UUID     // decision; nil = system
	Reason  string         // decision
	Notify  NotifyKind     // notify
	To      uuid.UUID      // notify recipient
	Ledger  LedgerKind     // penalty: doer_miss or backer_miss
	Penalty int64          // penalty: unclamped penalty_per_miss
}

func (e Effect) String() string {
	switch e.Kind {
	case EffectDecision:
		return "decision:" + string(e.Action)
	case EffectNotify:
		return "notify:" + string(e.Notify)
	case EffectPenalty:
		return fmt.Sprintf("penalty:%s:%d", e.Ledger, e.Penalty)
	default:
		return string(e.Kind)
	}
}

const minReasonRunes = 10

// Transition applies one event to a check-in at time now. It is the single source
// of truth for the state machine: the API and the settlement worker both call it
// (AGENTS.md invariant 3). On error the check-in is returned unchanged with no effects.
func Transition(ci CheckIn, ev Event, now time.Time, c TransitionCtx) (CheckIn, []Effect, error) {
	if ev.Kind == EventTick {
		return tick(ci, now, c)
	}
	if ci.IsFinal {
		return ci, nil, ErrInvalidTransition
	}
	t := &transition{ci: ci, next: ci, c: c, now: now, ev: ev}
	var err error
	switch ev.Kind {
	case EventSubmit:
		err = t.submit()
	case EventDeclareRest:
		err = t.declareRest()
	case EventApprove:
		err = t.approve()
	case EventReject:
		err = t.reject()
	case EventOverride:
		err = t.override()
	case EventDispute:
		err = t.dispute()
	case EventUphold, EventDismiss:
		err = t.resolve()
	default:
		err = fmt.Errorf("%w: unknown event %q", ErrInvalidTransition, ev.Kind)
	}
	if err != nil {
		return ci, nil, err
	}
	return t.next, t.effects, nil
}

type transition struct {
	ci, next CheckIn
	c        TransitionCtx
	now      time.Time
	ev       Event
	effects  []Effect
}

func (t *transition) actor() *uuid.UUID { id := t.ev.ActorID; return &id }

func (t *transition) decide(a DecisionAction, actor *uuid.UUID, reason string) {
	t.effects = append(t.effects, Effect{Kind: EffectDecision, Action: a, ActorID: actor, Reason: reason})
}

func (t *transition) notify(k NotifyKind, to uuid.UUID) {
	t.effects = append(t.effects, Effect{Kind: EffectNotify, Notify: k, To: to})
}

func (t *transition) penalize() {
	kind := LedgerDoerMiss
	if t.ci.MemberRole == RoleBacker {
		kind = LedgerBackerMiss
	}
	t.next.PenaltyApplied = true
	t.effects = append(t.effects, Effect{Kind: EffectPenalty, Ledger: kind, Penalty: t.c.Terms.Members[t.ci.MemberID].PenaltyPerMiss})
}

func (t *transition) reverseIfPenalized() {
	if t.ci.PenaltyApplied {
		t.next.PenaltyApplied = false
		t.effects = append(t.effects, Effect{Kind: EffectReversal})
	}
}

func validReason(r string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(r)) >= minReasonRunes
}

// ValidReason reports whether a written reason is long enough (SPEC §5). It is exported
// so the API can hold dispute resolutions to it for both outcomes, as SPEC §2 asks.
func ValidReason(r string) bool { return validReason(r) }

func ptr(t time.Time) *time.Time { return &t }

func (t *transition) submit() error {
	edit := t.ci.Status == StatusSubmitted
	if t.ci.Status != StatusOpen && t.ci.Status != StatusRejected && !edit {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.ci.MemberID {
		return ErrNotAllowed
	}
	if t.now.After(t.ci.SubmitDeadline) {
		return ErrDeadlinePassed
	}
	if !t.c.EvidenceOK {
		return ErrEvidenceInsufficient
	}
	if edit {
		t.notify(NotifyProofEdited, t.ci.ReviewerID)
		return nil
	}
	t.next.Status = StatusSubmitted
	t.next.SubmittedAt = ptr(t.now)
	t.next.ReviewDeadline = ptr(t.c.Terms.ReviewDeadline(t.ci.CutoffAt, t.now))
	t.next.DecidedAt, t.next.DisputeDeadline = nil, nil
	t.decide(ActionSubmit, t.actor(), "")
	t.notify(NotifyProofSubmitted, t.ci.ReviewerID)
	return nil
}

func (t *transition) declareRest() error {
	if t.ci.Status != StatusOpen {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.ci.MemberID {
		return ErrNotAllowed
	}
	// Rest must be declared in advance, never retroactively (SPEC §5).
	if !t.now.Before(t.ci.CutoffAt) {
		return ErrDeadlinePassed
	}
	if t.c.RestDaysUsed >= t.c.Terms.Members[t.ci.MemberID].RestDays {
		return ErrRestLimit
	}
	t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusRest, true, ptr(t.now)
	t.decide(ActionRest, t.actor(), "")
	t.effects = append(t.effects, Effect{Kind: EffectIncRestDays})
	t.notify(NotifyRestDeclared, t.ci.ReviewerID)
	return nil
}

func (t *transition) reviewable() error {
	if t.ci.Status != StatusSubmitted {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.ci.ReviewerID {
		return ErrNotAllowed
	}
	if t.ci.ReviewDeadline == nil || t.now.After(*t.ci.ReviewDeadline) {
		return ErrDeadlinePassed
	}
	return nil
}

func (t *transition) approve() error {
	if err := t.reviewable(); err != nil {
		return err
	}
	t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusApproved, true, ptr(t.now)
	t.decide(ActionApprove, t.actor(), "")
	t.notify(NotifyProofApproved, t.ci.MemberID)
	return nil
}

func (t *transition) reject() error {
	if err := t.reviewable(); err != nil {
		return err
	}
	if !validReason(t.ev.Reason) {
		return ErrReasonRequired
	}
	t.next.Status, t.next.DecidedAt = StatusRejected, ptr(t.now)
	t.next.DisputeDeadline = ptr(t.c.Terms.DisputeDeadline(t.now))
	t.decide(ActionReject, t.actor(), t.ev.Reason)
	t.notify(NotifyProofRejected, t.ci.MemberID)
	return nil
}

// overrideAllowed reports whether backer power can still apply to this check-in:
// only the backer, only on check-ins the backer reviews, within the pact's limit.
func overrideAllowed(ci CheckIn, c TransitionCtx) bool {
	return ci.ReviewerID == c.BackerID && c.OverridesUsed < c.Terms.MaxOverrides
}

func (t *transition) override() error {
	if t.ci.Status != StatusAutoApproved {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.c.BackerID || t.ev.ActorID != t.ci.ReviewerID {
		return ErrNotAllowed
	}
	if t.ci.OverrideDeadline == nil || t.now.After(*t.ci.OverrideDeadline) {
		return ErrDeadlinePassed
	}
	if !overrideAllowed(t.ci, t.c) {
		return ErrOverrideLimit
	}
	if !validReason(t.ev.Reason) {
		return ErrReasonRequired
	}
	// An override is final and cannot be disputed: that is the backer's power (ADR-0007).
	t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusRejected, true, ptr(t.now)
	t.decide(ActionOverride, t.actor(), t.ev.Reason)
	t.effects = append(t.effects, Effect{Kind: EffectIncOverrides})
	t.penalize()
	t.notify(NotifyProofOverridden, t.ci.MemberID)
	return nil
}

func (t *transition) dispute() error {
	if t.ci.Status != StatusRejected {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.ci.MemberID {
		return ErrNotAllowed
	}
	if t.ci.DisputeDeadline == nil || t.now.After(*t.ci.DisputeDeadline) {
		return ErrDeadlinePassed
	}
	if !validReason(t.ev.Reason) {
		return ErrReasonRequired
	}
	t.next.Status, t.next.DisputedAt = StatusDisputed, ptr(t.now)
	t.next.ResolutionDeadline = ptr(t.c.Terms.ResolutionDeadline(t.now))
	t.decide(ActionDispute, t.actor(), t.ev.Reason)
	t.notify(NotifyDisputeOpened, t.ci.ReviewerID)
	return nil
}

func (t *transition) resolve() error {
	if t.ci.Status != StatusDisputed {
		return ErrInvalidTransition
	}
	if t.ev.ActorID != t.ci.ReviewerID {
		return ErrNotAllowed
	}
	if t.ci.ResolutionDeadline == nil || t.now.After(*t.ci.ResolutionDeadline) {
		return ErrDeadlinePassed
	}
	if t.ev.Kind == EventUphold {
		t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusApproved, true, ptr(t.now)
		t.decide(ActionUphold, t.actor(), t.ev.Reason)
		t.reverseIfPenalized()
		t.notify(NotifyDisputeUpheld, t.ci.MemberID)
		return nil
	}
	if !validReason(t.ev.Reason) {
		return ErrReasonRequired
	}
	t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusRejected, true, ptr(t.now)
	t.decide(ActionDismiss, t.actor(), t.ev.Reason)
	t.penalize()
	t.notify(NotifyDisputeDismissed, t.ci.MemberID)
	return nil
}

// tick applies every deadline that has passed, chaining steps when the sweep runs
// late. Times are taken from the deadline itself, not from now, so sweep lag never
// shifts a window (SPEC §7).
func tick(ci CheckIn, now time.Time, c TransitionCtx) (CheckIn, []Effect, error) {
	t := &transition{ci: ci, next: ci, c: c, now: now}
	for range 4 { // at most open/submitted → auto_approved → final
		t.ci = t.next
		if !t.tickOnce() {
			break
		}
	}
	if t.effects == nil {
		t.effects = []Effect{}
	}
	return t.next, t.effects, nil
}

func (t *transition) tickOnce() bool {
	ci, now := t.ci, t.now
	if ci.IsFinal {
		return false
	}
	switch {
	case ci.Status == StatusOpen && now.After(ci.SubmitDeadline):
		t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusMissed, true, ptr(ci.SubmitDeadline)
		t.decide(ActionMissed, nil, "")
		t.penalize()
		t.notify(NotifyDayMissed, ci.MemberID)
	case ci.Status == StatusSubmitted && ci.ReviewDeadline != nil && now.After(*ci.ReviewDeadline):
		at := *ci.ReviewDeadline
		t.next.Status, t.next.DecidedAt = StatusAutoApproved, ptr(at)
		t.next.OverrideDeadline = ptr(t.c.Terms.OverrideDeadline(at))
		// Final at once when no override can ever apply (SPEC §5).
		t.next.IsFinal = !overrideAllowed(ci, t.c)
		t.decide(ActionAutoApprove, nil, "")
		t.notify(NotifyProofAutoApproved, ci.MemberID)
	case ci.Status == StatusAutoApproved && ci.OverrideDeadline != nil && now.After(*ci.OverrideDeadline):
		t.next.IsFinal = true
		t.decide(ActionFinalize, nil, "")
	case ci.Status == StatusRejected && ci.DisputeDeadline != nil && now.After(*ci.DisputeDeadline):
		t.next.IsFinal = true
		t.decide(ActionFinalize, nil, "")
		t.penalize()
		t.notify(NotifyRejectionFinal, ci.MemberID)
	case ci.Status == StatusDisputed && ci.ResolutionDeadline != nil && now.After(*ci.ResolutionDeadline):
		// An unresolved dispute is decided in the doer's favour (ADR-0007).
		t.next.Status, t.next.IsFinal, t.next.DecidedAt = StatusApproved, true, ptr(*ci.ResolutionDeadline)
		t.decide(ActionDisputeExpired, nil, "")
		t.reverseIfPenalized()
		t.notify(NotifyDisputeUpheld, ci.MemberID)
	default:
		return false
	}
	return true
}
