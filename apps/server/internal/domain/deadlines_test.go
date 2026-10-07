package domain

import (
	"testing"
	"time"
)

func TestCheckInDeadlinesAcrossIndonesianTimezones(t *testing.T) {
	cases := []struct {
		tz          string
		wantCutoffZ string // UTC instant of 2026-11-02 23:59 local
		wantSubmitZ string
	}{
		{"Asia/Jakarta", "2026-11-02T16:59:00Z", "2026-11-02T17:29:00Z"},  // WIB, UTC+7
		{"Asia/Makassar", "2026-11-02T15:59:00Z", "2026-11-02T16:29:00Z"}, // WITA, UTC+8
		{"Asia/Jayapura", "2026-11-02T14:59:00Z", "2026-11-02T15:29:00Z"}, // WIT, UTC+9
		{"UTC", "2026-11-02T23:59:00Z", "2026-11-03T00:29:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.tz, func(t *testing.T) {
			terms := sampleTerms()
			terms.Timezone = tc.tz
			cutoff, submit, err := terms.CheckInDeadlines(MustDate("2026-11-02"))
			if err != nil {
				t.Fatal(err)
			}
			if got := cutoff.UTC().Format(time.RFC3339); got != tc.wantCutoffZ {
				t.Errorf("cutoff: want %s, got %s", tc.wantCutoffZ, got)
			}
			if got := submit.UTC().Format(time.RFC3339); got != tc.wantSubmitZ {
				t.Errorf("submit deadline: want %s, got %s", tc.wantSubmitZ, got)
			}
		})
	}
}

func TestReviewDeadlineCountsFromCutoffForEarlySubmitters(t *testing.T) {
	terms := sampleTerms()
	cutoff := mustTime("2026-11-02T16:59:00Z")
	early := mustTime("2026-11-02T10:00:00Z")
	late := mustTime("2026-11-02T17:10:00Z") // inside grace
	if got := terms.ReviewDeadline(cutoff, early); !got.Equal(cutoff.Add(24 * time.Hour)) {
		t.Errorf("early submit: want cutoff+24h, got %s", got)
	}
	if got := terms.ReviewDeadline(cutoff, late); !got.Equal(late.Add(24 * time.Hour)) {
		t.Errorf("late submit: want submitted+24h, got %s", got)
	}
}

func TestWindowDeadlines(t *testing.T) {
	terms := sampleTerms()
	at := mustTime("2026-11-03T00:00:00Z")
	if got := terms.DisputeDeadline(at); !got.Equal(at.Add(24 * time.Hour)) {
		t.Errorf("dispute: %s", got)
	}
	if got := terms.ResolutionDeadline(at); !got.Equal(at.Add(48 * time.Hour)) {
		t.Errorf("resolution: %s", got)
	}
	if got := terms.OverrideDeadline(at); !got.Equal(at.Add(48 * time.Hour)) {
		t.Errorf("override: %s", got)
	}
}

func TestDateHelpers(t *testing.T) {
	d := MustDate("2026-12-31")
	if d.AddDays(1).String() != "2027-01-01" {
		t.Fatal("AddDays across year")
	}
	if _, err := ParseDate("2026-02-30"); err == nil {
		t.Fatal("invalid calendar date must fail")
	}
	if MustDate("2026-11-02").DaysUntil(MustDate("2026-11-29")) != 27 {
		t.Fatal("DaysUntil")
	}
	if !MustDate("2026-11-02").Before(MustDate("2026-11-03")) {
		t.Fatal("Before")
	}
}

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
