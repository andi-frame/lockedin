package domain

import "errors"

// Error is a domain error with a stable machine-readable code. The HTTP layer maps
// codes to problem+json responses, and the web app maps them to i18n messages.
type Error struct {
	Code string
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

var (
	ErrInvalidTerms         = &Error{"pact.invalid_terms", "invalid pact terms"}
	ErrInvalidTransition    = &Error{"checkin.invalid_transition", "this action is not possible in the check-in's current state"}
	ErrNotAllowed           = &Error{"checkin.not_allowed", "you are not allowed to do this on this check-in"}
	ErrDeadlinePassed       = &Error{"checkin.deadline_passed", "the deadline for this action has passed"}
	ErrReasonRequired       = &Error{"checkin.reason_required", "a reason of at least 10 characters is required"}
	ErrEvidenceInsufficient = &Error{"checkin.evidence_insufficient", "the proof does not meet the pact's evidence rules"}
	ErrOverrideLimit        = &Error{"checkin.override_limit", "no overrides left in this pact"}
	ErrRestLimit            = &Error{"checkin.rest_limit", "no rest days left in this pact"}
)

// CodeOf returns the stable code of a domain error, or "" for other errors.
func CodeOf(err error) string {
	var de *Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ""
}
