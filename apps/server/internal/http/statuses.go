package http

import "github.com/andi-frame/lockedin/apps/server/internal/http/api"

// statuses maps every contract error code to its HTTP status. It mirrors the table
// in the ErrorCode description of api/openapi.yaml, and spec_test.go fails when the
// two drift apart.
var statuses = map[api.ErrorCode]int{
	api.ValidationFailed:            400,
	api.AuthInvalidEmail:            400,
	api.AuthInvalidName:             400,
	api.AuthWeakPassword:            400,
	api.PactInvalidTerms:            400,
	api.PactTermsMembers:            400,
	api.ProofInvalidDoc:             400,
	api.CheckinReasonRequired:       400,
	api.UploadSizeMismatch:          400,
	api.AuthUnauthenticated:         401,
	api.AuthInvalidCredentials:      401,
	api.AuthCsrf:                    403,
	api.PactNotBacker:               403,
	api.CheckinNotAllowed:           403,
	api.NotFound:                    404,
	api.MethodNotAllowed:            405,
	api.AuthEmailTaken:              409,
	api.PactInvalidState:            409,
	api.PactTermsMismatch:           409,
	api.PactMemberMissing:           409,
	api.PactLimitReached:            409,
	api.CheckinInvalidTransition:    409,
	api.CheckinConflict:             409,
	api.CheckinOverrideLimit:        409,
	api.CheckinRestLimit:            409,
	api.UploadQuotaExceeded:         409,
	api.IdempotencyInProgress:       409,
	api.PactInviteInvalid:           410,
	api.UploadTooLarge:              413,
	api.RequestTooLarge:             413,
	api.UploadUnsupportedType:       415,
	api.RequestUnsupportedMediaType: 415,
	api.PactSignatureMismatch:       422,
	api.CheckinDeadlinePassed:       422,
	api.CheckinEvidenceInsufficient: 422,
	api.IdempotencyKeyReused:        422,
	api.RateLimited:                 429,
	api.AuthRateLimited:             429,
	api.ServerInternal:              500,
	api.UploadQueueBusy:             503,
	api.ServerUnavailable:           503,
}

// StatusFor returns the HTTP status for a contract error code.
func StatusFor(code string) (int, bool) {
	s, ok := statuses[api.ErrorCode(code)]
	return s, ok
}
