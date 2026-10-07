package domain

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFakeClock(t *testing.T) {
	c := NewFakeClock(mustTime("2026-11-02T00:00:00Z"))
	c.Advance(90 * time.Minute)
	if !c.Now().Equal(mustTime("2026-11-02T01:30:00Z")) {
		t.Fatal("Advance")
	}
	c.Set(mustTime("2027-01-01T00:00:00Z"))
	if c.Now().Year() != 2027 {
		t.Fatal("Set")
	}
	if SystemClock.Now(SystemClock{}).Location() != time.UTC {
		t.Fatal("system clock must be UTC")
	}
}

func TestDateConversions(t *testing.T) {
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	// 2026-11-02 18:00Z is already 2026-11-03 01:00 in Jakarta.
	if got := DateIn(mustTime("2026-11-02T18:00:00Z"), jkt); !got.Equal(NewDate(2026, time.November, 3)) {
		t.Fatalf("DateIn: %s", got)
	}
	if MustDate("2026-11-02").Time().Hour() != 0 {
		t.Fatal("Time is midnight UTC")
	}
	var d Date
	if err := d.UnmarshalJSON([]byte(`"2026-13-01"`)); err == nil {
		t.Fatal("bad month must fail")
	}
	if err := d.UnmarshalJSON([]byte(`20261101`)); err == nil {
		t.Fatal("non-string must fail")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustDate must panic on bad input")
		}
	}()
	MustDate("nope")
}

func TestCodeOf(t *testing.T) {
	wrapped := fmt.Errorf("service: %w", ErrOverrideLimit)
	if CodeOf(wrapped) != "checkin.override_limit" {
		t.Fatal("code of wrapped domain error")
	}
	if CodeOf(errors.New("boom")) != "" {
		t.Fatal("non-domain error has no code")
	}
	if ErrRestLimit.Error() == "" {
		t.Fatal("message")
	}
}

func TestEvidenceCheck(t *testing.T) {
	e := Evidence{MinAttachments: 1, MinWords: 20}
	if err := e.Check(25, 1, 0); err != nil {
		t.Fatalf("meets rules: %v", err)
	}
	for _, tc := range []struct{ words, ready, pending int }{{19, 1, 0}, {25, 0, 0}, {25, 1, 1}} {
		if err := e.Check(tc.words, tc.ready, tc.pending); !errors.Is(err, ErrEvidenceInsufficient) {
			t.Fatalf("%+v should fail", tc)
		}
	}
}

func TestTermsHelpersAndParsing(t *testing.T) {
	terms := sampleTerms()
	if id, ok := terms.Backer(); !ok || id != backerID {
		t.Fatal("Backer")
	}
	delete(terms.Members, backerID)
	if _, ok := terms.Backer(); ok {
		t.Fatal("no backer")
	}
	if _, err := terms.ReviewerOf(doerID); err == nil {
		t.Fatal("single-member pact has no reviewer")
	}
	if _, err := ParseTerms([]byte(`{"starts_on": 5}`)); !errors.Is(err, ErrInvalidTerms) {
		t.Fatalf("bad JSON must be ErrInvalidTerms, got %v", err)
	}
	bad := sampleTerms()
	bad.Timezone = "Nowhere/City"
	if _, _, err := bad.CheckInDeadlines(bad.StartsOn); err == nil {
		t.Fatal("bad timezone")
	}
	bad = sampleTerms()
	bad.CutoffLocalTime = "9:00"
	if _, _, err := bad.CheckInDeadlines(bad.StartsOn); err == nil {
		t.Fatal("bad cutoff")
	}
}

func TestMemberValidationEdgeCases(t *testing.T) {
	terms := sampleTerms()
	m := terms.Members[doerID]
	m.Role = "referee"
	terms.Members[doerID] = m
	if err := terms.Validate(); err == nil {
		t.Fatal("unknown role")
	}
	terms = sampleTerms()
	m = terms.Members[doerID]
	m.Evidence.MinAttachments = 11
	terms.Members[doerID] = m
	if err := terms.Validate(); err == nil {
		t.Fatal("evidence out of range")
	}
	terms = sampleTerms()
	m = terms.Members[doerID]
	m.Schedule = []int{1, 1}
	terms.Members[doerID] = m
	if err := terms.Validate(); err == nil {
		t.Fatal("repeated weekday")
	}
}

func TestWrongStateAndActorGuards(t *testing.T) {
	stranger := uuid.MustParse("00000000-0000-7000-8000-0000000000ff")
	cases := []struct {
		name  string
		start func(*testing.T) CheckIn
		ev    Event
		now   time.Time
		want  error
	}{
		{"unknown event", func(*testing.T) CheckIn { return openDoerCheckIn() }, Event{Kind: "teleport"}, morning, ErrInvalidTransition},
		{"rest on submitted", submitted, Event{Kind: EventDeclareRest, ActorID: doerID}, morning, ErrInvalidTransition},
		{"rest by reviewer", func(*testing.T) CheckIn { return openDoerCheckIn() }, Event{Kind: EventDeclareRest, ActorID: backerID}, morning, ErrNotAllowed},
		{"reject an open day", func(*testing.T) CheckIn { return openDoerCheckIn() }, Event{Kind: EventReject, ActorID: backerID, Reason: reason}, morning, ErrInvalidTransition},
		{"reject by stranger", submitted, Event{Kind: EventReject, ActorID: stranger, Reason: reason}, cutoff, ErrNotAllowed},
		{"reject after review deadline", submitted, Event{Kind: EventReject, ActorID: backerID, Reason: reason}, cutoff.Add(25 * time.Hour), ErrDeadlinePassed},
		{"override a submitted day", submitted, Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, cutoff, ErrInvalidTransition},
		{"dispute a submitted day", submitted, Event{Kind: EventDispute, ActorID: doerID, Reason: reason}, cutoff, ErrInvalidTransition},
		{"uphold a rejected day", rejected, Event{Kind: EventUphold, ActorID: backerID}, cutoff.Add(2 * time.Hour), ErrInvalidTransition},
		{"resolve after deadline", disputed, Event{Kind: EventUphold, ActorID: backerID}, cutoff.Add(60 * time.Hour), ErrDeadlinePassed},
		{"resubmit rejected after grace", rejected, Event{Kind: EventSubmit, ActorID: doerID}, deadline.Add(time.Hour), ErrDeadlinePassed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := Transition(tc.start(t), tc.ev, tc.now, ctx()); !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestOverrideOnBackersOwnCheckInIsNotAllowed(t *testing.T) {
	// The doer reviews the backer's check-in; backer power never applies to it.
	ci, _ := step(t, openBackerCheckIn(), Event{Kind: EventSubmit, ActorID: backerID}, morning, ctx())
	ci.Status, ci.IsFinal = StatusAutoApproved, false
	ci.OverrideDeadline = ptr(cutoff.Add(72 * time.Hour))
	if _, _, err := Transition(ci, Event{Kind: EventOverride, ActorID: backerID, Reason: reason}, cutoff.Add(30*time.Hour), ctx()); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("want ErrNotAllowed, got %v", err)
	}
}

func TestEffectStringFallback(t *testing.T) {
	if (Effect{Kind: EffectIncOverrides}).String() != "inc_overrides" {
		t.Fatal("plain kinds print as-is")
	}
}
