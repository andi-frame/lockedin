// Package service holds the use-cases. Every state change runs in one transaction
// that also writes its ledger rows, decisions, and outbox notifications.
package service

import "github.com/andi-frame/lockedin/apps/server/internal/domain"

var (
	// ErrNotFound is also returned to non-members so pact existence never leaks (invariant 8).
	ErrNotFound          = &domain.Error{Code: "not_found", Msg: "not found"}
	ErrPactState         = &domain.Error{Code: "pact.invalid_state", Msg: "the pact is not in a state that allows this"}
	ErrTermsMismatch     = &domain.Error{Code: "pact.terms_mismatch", Msg: "the terms changed; review them again before accepting"}
	ErrNotBacker         = &domain.Error{Code: "pact.not_backer", Msg: "only the backer can do this"}
	ErrInviteInvalid     = &domain.Error{Code: "pact.invite_invalid", Msg: "this invite link is invalid, used, or expired"}
	ErrPactLimit         = &domain.Error{Code: "pact.limit_reached", Msg: "you already have 10 pacts in progress"}
	ErrSignatureMismatch = &domain.Error{Code: "pact.signature_mismatch", Msg: "type your display name exactly to sign"}
	ErrMemberMissing     = &domain.Error{Code: "pact.member_missing", Msg: "the second member has not joined yet"}
	ErrTermsShape        = &domain.Error{Code: "pact.terms_members", Msg: "terms must name the backer and one doer slot"}
)

const maxOpenPacts = 10
