package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Day under test: Monday 2026-11-02 in Asia/Jakarta.
//
//	cutoff_at       = 16:59Z, submit_deadline = 17:29Z (30 min grace)
var (
	cutoff   = mustTime("2026-11-02T16:59:00Z")
	deadline = mustTime("2026-11-02T17:29:00Z")
	morning  = mustTime("2026-11-02T03:00:00Z")
	reason   = "foto tidak menunjukkan soal yang dikerjakan"
)

func openDoerCheckIn() CheckIn {
	return CheckIn{
		ID:             uuid.MustParse("00000000-0000-7000-8000-0000000000c1"),
		MemberID:       doerID,
		ReviewerID:     backerID,
		MemberRole:     RoleDoer,
		Status:         StatusOpen,
		CutoffAt:       cutoff,
		SubmitDeadline: deadline,
	}
}

func openBackerCheckIn() CheckIn {
	ci := openDoerCheckIn()
	ci.MemberID, ci.ReviewerID, ci.MemberRole = backerID, doerID, RoleBacker
	return ci
}

func ctx() TransitionCtx {
	return TransitionCtx{Terms: sampleTerms(), BackerID: backerID, EvidenceOK: true}
}

// step applies one event and fails the test on error.
func step(t *testing.T, ci CheckIn, ev Event, now time.Time, c TransitionCtx) (CheckIn, []Effect) {
	t.Helper()
	next, effects, err := Transition(ci, ev, now, c)
	if err != nil {
		t.Fatalf("%s at %s: unexpected error %v", ev.Kind, now.Format(time.RFC3339), err)
	}
	return next, effects
}

func submitted(t *testing.T) CheckIn {
	ci, _ := step(t, openDoerCheckIn(), Event{Kind: EventSubmit, ActorID: doerID}, morning, ctx())
	return ci
}

func rejected(t *testing.T) CheckIn {
	ci, _ := step(t, submitted(t), Event{Kind: EventReject, ActorID: backerID, Reason: reason}, cutoff.Add(time.Hour), ctx())
	return ci
}

func disputed(t *testing.T) CheckIn {
	ci := rejected(t)
	ci, _ = step(t, ci, Event{Kind: EventDispute, ActorID: doerID, Reason: "semua 50 soal ada di lampiran kedua"}, cutoff.Add(2*time.Hour), ctx())
	return ci
}

func autoApproved(t *testing.T) CheckIn {
	ci, _ := step(t, submitted(t), Event{Kind: EventTick}, cutoff.Add(25*time.Hour), ctx())
	return ci
}

func kinds(effects []Effect) []string {
	out := make([]string, 0, len(effects))
	for _, e := range effects {
		out = append(out, e.String())
	}
	return out
}

func TestHappyPathSubmitApprove(t *testing.T) {
	ci, eff := step(t, openDoerCheckIn(), Event{Kind: EventSubmit, ActorID: doerID}, morning, ctx())
	if ci.Status != StatusSubmitted || ci.SubmittedAt == nil || !ci.SubmittedAt.Equal(morning) {
		t.Fatalf("submit: %+v", ci)
	}
	if !ci.ReviewDeadline.Equal(cutoff.Add(24 * time.Hour)) {
		t.Fatalf("review deadline counts from cutoff, got %s", ci.ReviewDeadline)
	}
	if want := []string{"decision:submit", "notify:proof_submitted"}; !reflect.DeepEqual(kinds(eff), want) {
		t.Fatalf("effects: want %v, got %v", want, kinds(eff))
	}

	ci, eff = step(t, ci, Event{Kind: EventApprove, ActorID: backerID}, cutoff.Add(time.Hour), ctx())
	if ci.Status != StatusApproved || !ci.IsFinal || ci.DecidedAt == nil {
		t.Fatalf("approve: %+v", ci)
	}
	if want := []string{"decision:approve", "notify:proof_approved"}; !reflect.DeepEqual(kinds(eff), want) {
		t.Fatalf("effects: want %v, got %v", want, kinds(eff))
	}
}

func TestTransitions(t *testing.T) {
	cases := []struct {
		name      string
		start     func(t *testing.T) CheckIn
		ctx       func(TransitionCtx) TransitionCtx
		event     Event
		now       time.Time
		status    Status
		final     bool
		effects   []string
		wantError error
	}{
		// --- open ---
		{name: "submit within grace", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventSubmit, ActorID: doerID}, now: deadline,
			status: StatusSubmitted, effects: []string{"decision:submit", "notify:proof_submitted"}},
		{name: "submit after grace", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventSubmit, ActorID: doerID}, now: deadline.Add(time.Second), wantError: ErrDeadlinePassed},
		{name: "submit by reviewer", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventSubmit, ActorID: backerID}, now: morning, wantError: ErrNotAllowed},
		{name: "submit without enough evidence", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			ctx:   func(c TransitionCtx) TransitionCtx { c.EvidenceOK = false; return c },
			event: Event{Kind: EventSubmit, ActorID: doerID}, now: morning, wantError: ErrEvidenceInsufficient},
		{name: "declare rest before cutoff", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventDeclareRest, ActorID: doerID}, now: morning,
			status: StatusRest, final: true, effects: []string{"decision:rest", "inc_rest_days", "notify:rest_declared"}},
		{name: "declare rest after cutoff", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventDeclareRest, ActorID: doerID}, now: cutoff, wantError: ErrDeadlinePassed},
		{name: "declare rest over allowance", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			ctx:   func(c TransitionCtx) TransitionCtx { c.RestDaysUsed = 2; return c },
			event: Event{Kind: EventDeclareRest, ActorID: doerID}, now: morning, wantError: ErrRestLimit},
		{name: "tick before deadline is a no-op", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventTick}, now: deadline, status: StatusOpen},
		{name: "doer misses: penalty", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventTick}, now: deadline.Add(time.Second),
			status: StatusMissed, final: true, effects: []string{"decision:missed", "penalty:doer_miss:50", "notify:day_missed"}},
		{name: "backer misses: bonus to pot", start: func(*testing.T) CheckIn { return openBackerCheckIn() },
			event: Event{Kind: EventTick}, now: deadline.Add(time.Minute),
			status: StatusMissed, final: true, effects: []string{"decision:missed", "penalty:backer_miss:50", "notify:day_missed"}},
		{name: "approve an open check-in", start: func(*testing.T) CheckIn { return openDoerCheckIn() },
			event: Event{Kind: EventApprove, ActorID: backerID}, now: morning, wantError: ErrInvalidTransition},

		// --- submitted ---
		{name: "edit proof before deadline", start: submitted,
			event: Event{Kind: EventSubmit, ActorID: doerID}, now: cutoff,
			status: StatusSubmitted, effects: []string{"notify:proof_edited"}},
		{name: "edit proof after deadline", start: submitted,
			event: Event{Kind: EventSubmit, ActorID: doerID}, now: deadline.Add(time.Minute), wantError: ErrDeadlinePassed},
		{name: "doer approves own proof", start: submitted,
			event: Event{Kind: EventApprove, ActorID: doerID}, now: cutoff, wantError: ErrNotAllowed},
		{name: "approve after review deadline", start: submitted,
			event: Event{Kind: EventApprove, ActorID: backerID}, now: cutoff.Add(24*time.Hour + time.Second), wantError: ErrDeadlinePassed},
		{name: "reject without reason", start: submitted,
			event: Event{Kind: EventReject, ActorID: backerID, Reason: "jelek"}, now: cutoff, wantError: ErrReasonRequired},
		{name: "reject with reason", start: submitted,
			event: Event{Kind: EventReject, ActorID: backerID, Reason: reason}, now: cutoff.Add(time.Hour),
			status: StatusRejected, effects: []string{"decision:reject", "notify:proof_rejected"}},
		{name: "review timeout auto-approves", start: submitted,
			event: Event{Kind: EventTick}, now: cutoff.Add(24*time.Hour + time.Minute),
			status: StatusAutoApproved, effects: []string{"decision:auto_approve", "notify:proof_auto_approved"}},
		{name: "auto-approve is final when no overrides remain", start: submitted,
			ctx:   func(c TransitionCtx) TransitionCtx { c.OverridesUsed = 3; return c },
			event: Event{Kind: EventTick}, now: cutoff.Add(25 * time.Hour),
			status: StatusAutoApproved, final: true, effects: []string{"decision:auto_approve", "notify:proof_auto_approved"}},
		{name: "backer's own auto-approval is final at once (no override on own check-in)",
			start: func(t *testing.T) CheckIn {
				ci, _ := step(t, openBackerCheckIn(), Event{Kind: EventSubmit, ActorID: backerID}, morning, ctx())
				return ci
			},
			event: Event{Kind: EventTick}, now: cutoff.Add(25 * time.Hour),
			status: StatusAutoApproved, final: true, effects: []string{"decision:auto_approve", "notify:proof_auto_approved"}},
		{name: "lagging sweep chains to final auto-approval", start: submitted,
			event: Event{Kind: EventTick}, now: cutoff.Add(24*time.Hour + 48*time.Hour + time.Minute),
			status: StatusAutoApproved, final: true, effects: []string{"decision:auto_approve", "notify:proof_auto_approved", "decision:finalize"}},

		// --- rejected ---
		{name: "resubmit after rejection inside grace", start: func(t *testing.T) CheckIn {
			ci, _ := step(t, submitted(t), Event{Kind: EventReject, ActorID: backerID, Reason: reason}, cutoff.Add(-time.Hour), ctx())
			return ci
		}, event: Event{Kind: EventSubmit, ActorID: doerID}, now: cutoff,
			status: StatusSubmitted, effects: []string{"decision:submit", "notify:proof_submitted"}},
		{name: "dispute a rejection", start: rejected,
			event: Event{Kind: EventDispute, ActorID: doerID, Reason: "lampiran kedua berisi semua soal"}, now: cutoff.Add(2 * time.Hour),
			status: StatusDisputed, effects: []string{"decision:dispute", "notify:dispute_opened"}},
		{name: "dispute needs a reason", start: rejected,
			event: Event{Kind: EventDispute, ActorID: doerID}, now: cutoff.Add(2 * time.Hour), wantError: ErrReasonRequired},
		{name: "dispute too late", start: rejected,
			event: Event{Kind: EventDispute, ActorID: doerID, Reason: "lampiran kedua berisi semua soal"}, now: cutoff.Add(26 * time.Hour), wantError: ErrDeadlinePassed},
		{name: "reviewer cannot dispute", start: rejected,
			event: Event{Kind: EventDispute, ActorID: backerID, Reason: "lampiran kedua berisi semua soal"}, now: cutoff.Add(2 * time.Hour), wantError: ErrNotAllowed},
		{name: "undisputed rejection becomes final with penalty", start: rejected,
			event: Event{Kind: EventTick}, now: cutoff.Add(26 * time.Hour),
			status: StatusRejected, final: true, effects: []string{"decision:finalize", "penalty:doer_miss:50", "notify:rejection_final"}},

		// --- disputed ---
		{name: "backer upholds dispute", start: disputed,
			event: Event{Kind: EventUphold, ActorID: backerID}, now: cutoff.Add(3 * time.Hour),
			status: StatusApproved, final: true, effects: []string{"decision:uphold", "notify:dispute_upheld"}},
		{name: "backer dismisses dispute: penalty", start: disputed,
			event: Event{Kind: EventDismiss, ActorID: backerID, Reason: "lampiran kedua juga bukan soal UTBK"}, now: cutoff.Add(3 * time.Hour),
			status: StatusRejected, final: true, effects: []string{"decision:dismiss", "penalty:doer_miss:50", "notify:dispute_dismissed"}},
		{name: "dismiss needs a reason", start: disputed,
			event: Event{Kind: EventDismiss, ActorID: backerID}, now: cutoff.Add(3 * time.Hour), wantError: ErrReasonRequired},
		{name: "doer cannot resolve own dispute", start: disputed,
			event: Event{Kind: EventUphold, ActorID: doerID}, now: cutoff.Add(3 * time.Hour), wantError: ErrNotAllowed},
		{name: "unresolved dispute expires in doer's favour", start: disputed,
			event: Event{Kind: EventTick}, now: cutoff.Add(2*time.Hour + 48*time.Hour + time.Second),
			status: StatusApproved, final: true, effects: []string{"decision:dispute_expired", "notify:dispute_upheld"}},

		// --- auto_approved: backer power ---
		{name: "backer overrides auto-approval", start: autoApproved,
			event: Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, now: cutoff.Add(30 * time.Hour),
			status: StatusRejected, final: true, effects: []string{"decision:override", "inc_overrides", "penalty:doer_miss:50", "notify:proof_overridden"}},
		{name: "override needs a reason", start: autoApproved,
			event: Event{Kind: EventOverride, ActorID: backerID}, now: cutoff.Add(30 * time.Hour), wantError: ErrReasonRequired},
		{name: "override limit reached", start: autoApproved,
			ctx:   func(c TransitionCtx) TransitionCtx { c.OverridesUsed = 3; return c },
			event: Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, now: cutoff.Add(30 * time.Hour), wantError: ErrOverrideLimit},
		{name: "override after window", start: autoApproved,
			event: Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, now: cutoff.Add(24*time.Hour + 48*time.Hour + time.Second), wantError: ErrDeadlinePassed},
		{name: "doer cannot override", start: autoApproved,
			event: Event{Kind: EventOverride, ActorID: doerID, Reason: reason}, now: cutoff.Add(30 * time.Hour), wantError: ErrNotAllowed},
		{name: "override window closes: final, no coins", start: autoApproved,
			event: Event{Kind: EventTick}, now: cutoff.Add(24*time.Hour + 48*time.Hour + time.Second),
			status: StatusAutoApproved, final: true, effects: []string{"decision:finalize"}},
		{name: "overridden check-in cannot be disputed", start: func(t *testing.T) CheckIn {
			ci, _ := step(t, autoApproved(t), Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, cutoff.Add(30*time.Hour), ctx())
			return ci
		}, event: Event{Kind: EventDispute, ActorID: doerID, Reason: "saya keberatan dengan pembatalan ini"}, now: cutoff.Add(31 * time.Hour), wantError: ErrInvalidTransition},

		// --- final states reject everything except a no-op tick ---
		{name: "final tick is a no-op", start: func(t *testing.T) CheckIn {
			ci, _ := step(t, openDoerCheckIn(), Event{Kind: EventTick}, deadline.Add(time.Hour), ctx())
			return ci
		}, event: Event{Kind: EventTick}, now: deadline.Add(100 * time.Hour), status: StatusMissed, final: true},
		{name: "cannot submit a missed day", start: func(t *testing.T) CheckIn {
			ci, _ := step(t, openDoerCheckIn(), Event{Kind: EventTick}, deadline.Add(time.Hour), ctx())
			return ci
		}, event: Event{Kind: EventSubmit, ActorID: doerID}, now: deadline.Add(2 * time.Hour), wantError: ErrInvalidTransition},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ctx()
			if tc.ctx != nil {
				c = tc.ctx(c)
			}
			start := tc.start(t)
			got, effects, err := Transition(start, tc.event, tc.now, c)
			if tc.wantError != nil {
				if !errors.Is(err, tc.wantError) {
					t.Fatalf("want error %v, got %v", tc.wantError, err)
				}
				if !reflect.DeepEqual(got, start) || len(effects) != 0 {
					t.Fatal("a failed transition must return the check-in unchanged and no effects")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Status != tc.status || got.IsFinal != tc.final {
				t.Fatalf("want %s final=%v, got %s final=%v", tc.status, tc.final, got.Status, got.IsFinal)
			}
			if !reflect.DeepEqual(kinds(effects), nilIfEmpty(tc.effects)) {
				t.Fatalf("effects: want %v, got %v", tc.effects, kinds(effects))
			}
		})
	}
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return []string{}
	}
	return s
}

func TestDeadlinesUseTheDeadlineInstantNotSweepTime(t *testing.T) {
	ci := submitted(t)
	late := cutoff.Add(30 * time.Hour) // sweep ran 6h late
	got, _ := step(t, ci, Event{Kind: EventTick}, late, ctx())
	wantOverride := ci.ReviewDeadline.Add(48 * time.Hour)
	if got.OverrideDeadline == nil || !got.OverrideDeadline.Equal(wantOverride) {
		t.Fatalf("override window must start at the review deadline, got %v", got.OverrideDeadline)
	}
	if got.DecidedAt == nil || !got.DecidedAt.Equal(*ci.ReviewDeadline) {
		t.Fatalf("auto-approval time is the review deadline, got %v", got.DecidedAt)
	}
}

func TestPenaltyAppliedFlagAndReversal(t *testing.T) {
	// A penalised check-in that is later approved writes a reversal (SPEC §6).
	ci := disputed(t)
	ci.PenaltyApplied = true
	got, effects := step(t, ci, Event{Kind: EventUphold, ActorID: backerID}, cutoff.Add(3*time.Hour), ctx())
	if got.Status != StatusApproved {
		t.Fatal("uphold → approved")
	}
	if want := []string{"decision:uphold", "reversal", "notify:dispute_upheld"}; !reflect.DeepEqual(kinds(effects), want) {
		t.Fatalf("want %v, got %v", want, kinds(effects))
	}

	missed, _ := step(t, openDoerCheckIn(), Event{Kind: EventTick}, deadline.Add(time.Hour), ctx())
	if !missed.PenaltyApplied {
		t.Fatal("a penalised check-in must be flagged")
	}
}

func TestDecisionEffectsCarryActorAndReason(t *testing.T) {
	_, effects := step(t, submitted(t), Event{Kind: EventReject, ActorID: backerID, Reason: reason}, cutoff, ctx())
	d := effects[0]
	if d.Kind != EffectDecision || d.Action != ActionReject || d.ActorID == nil || *d.ActorID != backerID || d.Reason != reason {
		t.Fatalf("decision effect: %+v", d)
	}
	_, effects = step(t, submitted(t), Event{Kind: EventTick}, cutoff.Add(25*time.Hour), ctx())
	if effects[0].ActorID != nil {
		t.Fatal("system decisions have no actor")
	}
	n := effects[1]
	if n.Kind != EffectNotify || n.To != doerID {
		t.Fatalf("auto-approval notifies the doer: %+v", n)
	}
}
