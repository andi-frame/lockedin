package domain

import "time"

// StartHasPassed reports whether the pact's start date is before today in the pact's own time
// zone (SPEC §3). The start day itself still counts as not passed: its cutoff is ahead of us.
// Scheduling a pact after this would generate check-ins that are overdue at once, and the next
// sweep would move coins for days nobody could have met.
func StartHasPassed(t Terms, now time.Time) (bool, error) {
	loc, err := t.Location()
	if err != nil {
		return false, err
	}
	return t.StartsOn.Before(DateIn(now, loc)), nil
}
