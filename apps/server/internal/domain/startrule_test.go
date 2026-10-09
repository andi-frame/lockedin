package domain

import (
	"testing"
	"time"
)

// SPEC §3: a pact cannot be proposed or signed once its start date has passed in its own time
// zone, because the days before today would be overdue the moment the pact is scheduled.
func TestStartHasPassed(t *testing.T) {
	jakarta := Terms{Timezone: "Asia/Jakarta", StartsOn: MustDate("2026-11-02")}
	utcT := time.Date
	tests := []struct {
		name  string
		terms Terms
		now   time.Time
		want  bool
	}{
		{"the day before", jakarta, utcT(2026, 11, 1, 3, 0, 0, 0, time.UTC), false},
		{"the start day itself is still fine", jakarta, utcT(2026, 11, 2, 3, 0, 0, 0, time.UTC), false},
		{"the end of the start day in its zone", jakarta, utcT(2026, 11, 2, 16, 59, 0, 0, time.UTC), false},
		{"the day after in its zone, though still the 2nd in UTC", jakarta, utcT(2026, 11, 2, 17, 0, 0, 0, time.UTC), true},
		{"a week later", jakarta, utcT(2026, 11, 9, 3, 0, 0, 0, time.UTC), true},
		{"a zone behind UTC is still on the start day", Terms{Timezone: "America/Los_Angeles", StartsOn: MustDate("2026-11-02")}, utcT(2026, 11, 3, 5, 0, 0, 0, time.UTC), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := StartHasPassed(tc.terms, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("StartHasPassed = %v, want %v", got, tc.want)
			}
		})
	}
	if _, err := StartHasPassed(Terms{Timezone: "Mars/Olympus", StartsOn: MustDate("2026-11-02")}, time.Now()); err == nil {
		t.Error("an unknown zone must be an error, not a guess")
	}
}
