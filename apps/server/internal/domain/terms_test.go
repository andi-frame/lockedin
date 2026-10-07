package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	backerID = uuid.MustParse("00000000-0000-7000-8000-00000000000a")
	doerID   = uuid.MustParse("00000000-0000-7000-8000-00000000000b")
)

func i64(v int64) *int64 { return &v }

// sampleTerms is the worked example from docs/PLAN.md: 4 weeks from Monday 2026-11-02.
func sampleTerms() Terms {
	return Terms{
		Version:                1,
		Timezone:               "Asia/Jakarta",
		StartsOn:               MustDate("2026-11-02"),
		EndsOn:                 MustDate("2026-11-29"),
		CutoffLocalTime:        "23:59",
		GraceMinutes:           30,
		CoinRateIDR:            1000,
		InitialPot:             1000,
		PotFloor:               0,
		PotCap:                 i64(1500),
		ReviewWindowHours:      24,
		DisputeWindowHours:     24,
		DisputeResolutionHours: 48,
		OverrideWindowHours:    48,
		MaxOverrides:           3,
		BackerCommits:          true,
		Members: map[uuid.UUID]MemberTerms{
			backerID: {Role: RoleBacker, Commitment: "Belajar Kalkulus 2 jam", Schedule: []int{1, 2, 3, 4, 5}, PenaltyPerMiss: 50, RestDays: 2},
			doerID:   {Role: RoleDoer, Commitment: "Latihan soal UTBK 50 soal", Schedule: []int{1, 2, 3, 4, 5, 6}, PenaltyPerMiss: 50, RestDays: 2, Evidence: Evidence{MinAttachments: 1, MinWords: 20}},
		},
	}
}

func TestTermsValidateAcceptsSample(t *testing.T) {
	if err := sampleTerms().Validate(); err != nil {
		t.Fatalf("sample terms should be valid: %v", err)
	}
}

func TestTermsValidateRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Terms)
		want   string
	}{
		{"unknown timezone", func(t *Terms) { t.Timezone = "Mars/Olympus" }, "timezone"},
		{"bad cutoff", func(t *Terms) { t.CutoffLocalTime = "25:00" }, "cutoff_local_time"},
		{"ends before start", func(t *Terms) { t.EndsOn = t.StartsOn }, "ends_on"},
		{"too long", func(t *Terms) { t.EndsOn = t.StartsOn.AddDays(367) }, "366"},
		{"grace too long", func(t *Terms) { t.GraceMinutes = 181 }, "grace_minutes"},
		{"review window zero", func(t *Terms) { t.ReviewWindowHours = 0 }, "review_window_hours"},
		{"dispute window too long", func(t *Terms) { t.DisputeWindowHours = 73 }, "dispute_window_hours"},
		{"too many overrides", func(t *Terms) { t.MaxOverrides = 11 }, "max_overrides"},
		{"no initial pot", func(t *Terms) { t.InitialPot = 0 }, "initial_pot"},
		{"cap below initial", func(t *Terms) { t.PotCap = i64(999) }, "pot_cap"},
		{"floor not below initial", func(t *Terms) { t.PotFloor = 1000 }, "pot_floor"},
		{"coin rate", func(t *Terms) { t.CoinRateIDR = 0 }, "coin_rate_idr"},
		{"one member", func(t *Terms) { delete(t.Members, backerID) }, "two members"},
		{"two doers", func(t *Terms) {
			m := t.Members[backerID]
			m.Role = RoleDoer
			t.Members[backerID] = m
		}, "one backer"},
		{"doer without schedule", func(t *Terms) {
			m := t.Members[doerID]
			m.Schedule = nil
			t.Members[doerID] = m
		}, "schedule"},
		{"weekday out of range", func(t *Terms) {
			m := t.Members[doerID]
			m.Schedule = []int{0, 8}
			t.Members[doerID] = m
		}, "weekday"},
		{"backer schedule without backer_commits", func(t *Terms) { t.BackerCommits = false }, "backer_commits"},
		{"zero penalty", func(t *Terms) {
			m := t.Members[doerID]
			m.PenaltyPerMiss = 0
			t.Members[doerID] = m
		}, "penalty_per_miss"},
		{"empty commitment", func(t *Terms) {
			m := t.Members[doerID]
			m.Commitment = "  "
			t.Members[doerID] = m
		}, "commitment"},
		{"negative rest days", func(t *Terms) {
			m := t.Members[doerID]
			m.RestDays = -1
			t.Members[doerID] = m
		}, "rest_days"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			terms := sampleTerms()
			tc.mutate(&terms)
			err := terms.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
			if !errors.Is(err, ErrInvalidTerms) {
				t.Fatalf("want ErrInvalidTerms, got %v", err)
			}
		})
	}
}

func TestBackerWithoutCommitmentIsValid(t *testing.T) {
	terms := sampleTerms()
	terms.BackerCommits = false
	m := terms.Members[backerID]
	m.Schedule = nil
	m.PenaltyPerMiss = 0
	terms.Members[backerID] = m
	if err := terms.Validate(); err != nil {
		t.Fatalf("backer who does not commit should be valid: %v", err)
	}
}

func TestTermsHashIsStableAndSensitive(t *testing.T) {
	a, b := sampleTerms(), sampleTerms()
	ha, err := a.Hash()
	if err != nil {
		t.Fatal(err)
	}
	hb, _ := b.Hash()
	if ha != hb || len(ha) != 64 {
		t.Fatalf("hash should be stable 64-hex, got %q vs %q", ha, hb)
	}
	b.GraceMinutes = 31
	hc, _ := b.Hash()
	if hc == ha {
		t.Fatal("hash must change when terms change")
	}
}

func TestTermsJSONRoundTrip(t *testing.T) {
	a := sampleTerms()
	raw, err := a.MarshalCanonical()
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseTerms(raw)
	if err != nil {
		t.Fatal(err)
	}
	ha, _ := a.Hash()
	hb, _ := b.Hash()
	if ha != hb {
		t.Fatal("round trip changed the hash")
	}
	if !strings.Contains(string(raw), `"starts_on":"2026-11-02"`) {
		t.Fatalf("dates must serialise as YYYY-MM-DD: %s", raw)
	}
}

func TestScheduledDates(t *testing.T) {
	terms := sampleTerms()
	if got := len(terms.ScheduledDates(doerID)); got != 24 {
		t.Fatalf("doer Mon-Sat over 4 weeks: want 24, got %d", got)
	}
	if got := len(terms.ScheduledDates(backerID)); got != 20 {
		t.Fatalf("backer Mon-Fri over 4 weeks: want 20, got %d", got)
	}
	terms.BackerCommits = false
	if got := len(terms.ScheduledDates(backerID)); got != 0 {
		t.Fatalf("non-committing backer: want 0, got %d", got)
	}
	first := sampleTerms().ScheduledDates(doerID)[0]
	if first.String() != "2026-11-02" || first.Weekday() != time.Monday {
		t.Fatalf("first date wrong: %s %s", first, first.Weekday())
	}
}

func TestReviewerOf(t *testing.T) {
	terms := sampleTerms()
	if r, _ := terms.ReviewerOf(doerID); r != backerID {
		t.Fatal("backer reviews the doer")
	}
	if r, _ := terms.ReviewerOf(backerID); r != doerID {
		t.Fatal("doer reviews the backer")
	}
	if _, err := terms.ReviewerOf(uuid.New()); err == nil {
		t.Fatal("non-member has no reviewer")
	}
}
