package domain

import (
	"errors"
	"fmt"
	"sort"
)

// ErrInvalidEmailKind is returned when a preference names a kind that cannot be switched off.
// The HTTP layer reports it as validation.failed.
var ErrInvalidEmailKind = errors.New("this email cannot be switched off")

// SwitchableEmailKinds are the notification kinds whose email a person can turn off. SPEC §9:
// these say what the app already shows, and missing one costs nothing. Everything else that is
// mailed (invites, a dispute opened against you, a settled pact) stays on, because a deadline or
// a payout can depend on seeing it. The in-app inbox is never switched off.
var switchableEmailKinds = map[string]bool{
	"terms_changed":       true,
	"terms_signed":        true,
	"proof_submitted":     true,
	"proof_rejected":      true,
	"proof_overridden":    true,
	"proof_auto_approved": true,
}

// IsSwitchableEmailKind reports whether the email for a kind can be turned off.
func IsSwitchableEmailKind(kind string) bool { return switchableEmailKinds[kind] }

// NormaliseEmailOff validates a list of kinds a person has switched off and returns it sorted and
// without duplicates (never nil, so it stores as an empty array).
func NormaliseEmailOff(kinds []string) ([]string, error) {
	seen := make(map[string]bool, len(kinds))
	out := make([]string, 0, len(kinds))
	for _, k := range kinds {
		if !IsSwitchableEmailKind(k) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidEmailKind, k)
		}
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out, nil
}
