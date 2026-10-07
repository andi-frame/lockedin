package http

import (
	"context"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

// The upload operations are in the contract, but they need the BlobStore and the media
// worker (PLAN 4.1 and 4.2). Until then they answer 503 server.unavailable so a client
// can tell "not built yet" from a bug, and nothing is half-implemented behind them.
func uploadsUnavailable() error {
	return problemError(api.ServerUnavailable, "uploads are not available yet")
}

func (h *Handlers) CreateUpload(context.Context, api.CreateUploadRequestObject) (api.CreateUploadResponseObject, error) {
	return nil, uploadsUnavailable()
}

func (h *Handlers) CompleteUpload(context.Context, api.CompleteUploadRequestObject) (api.CompleteUploadResponseObject, error) {
	return nil, uploadsUnavailable()
}

func (h *Handlers) GetAttachment(context.Context, api.GetAttachmentRequestObject) (api.GetAttachmentResponseObject, error) {
	return nil, uploadsUnavailable()
}
