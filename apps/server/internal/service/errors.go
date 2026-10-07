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
	ErrNotDoer           = &domain.Error{Code: "pact.not_doer", Msg: "only the doer can do this"}
	ErrInviteInvalid     = &domain.Error{Code: "pact.invite_invalid", Msg: "this invite link is invalid, used, or expired"}
	ErrPactLimit         = &domain.Error{Code: "pact.limit_reached", Msg: "you already have 10 pacts in progress"}
	ErrSignatureMismatch = &domain.Error{Code: "pact.signature_mismatch", Msg: "type your display name exactly to sign"}
	ErrMemberMissing     = &domain.Error{Code: "pact.member_missing", Msg: "the second member has not joined yet"}
	ErrTermsShape        = &domain.Error{Code: "pact.terms_members", Msg: "terms must name the backer and one doer slot"}

	// Uploads (SPEC §8). The codes are the contract's.
	ErrUploadTooLarge     = &domain.Error{Code: "upload.too_large", Msg: "the file is larger than the limit for its kind"}
	ErrUploadUnsupported  = &domain.Error{Code: "upload.unsupported_type", Msg: "this kind or type of file is not accepted"}
	ErrUploadQuota        = &domain.Error{Code: "upload.quota_exceeded", Msg: "this pact has used up its storage quota"}
	ErrUploadQueueBusy    = &domain.Error{Code: "upload.queue_busy", Msg: "uploads are busy right now; try again in a moment"}
	ErrUploadSizeMismatch = &domain.Error{Code: "upload.size_mismatch", Msg: "the uploaded file does not match the size that was declared"}
	ErrUploadsOff         = &domain.Error{Code: "server.unavailable", Msg: "uploads are not available"}
)

const maxOpenPacts = 10
