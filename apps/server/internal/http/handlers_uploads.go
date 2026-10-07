package http

import (
	"context"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// The three upload operations. The bytes never pass through here: the browser PUTs straight to
// storage with the presigned URL and calls completeUpload afterwards (ARCHITECTURE §6).

func (h *Handlers) CreateUpload(ctx context.Context, req api.CreateUploadRequestObject) (api.CreateUploadResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || b.Bytes < 1 { // the generated server does not enforce the schema's minimum
		return nil, problemError(api.ValidationFailed, "bytes must be at least 1")
	}
	in, err := h.svc.CreateUpload(ctx, user, service.UploadInput{PactID: b.PactId, Kind: string(b.Kind), Mime: b.Mime, Bytes: b.Bytes})
	if err != nil {
		return nil, err
	}
	return api.CreateUpload201JSONResponse{AttachmentId: in.AttachmentID, PutUrl: in.PutURL, Headers: in.Headers, ExpiresAt: in.ExpiresAt}, nil
}

func (h *Handlers) CompleteUpload(ctx context.Context, req api.CompleteUploadRequestObject) (api.CompleteUploadResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	a, err := h.svc.CompleteUpload(ctx, user, req.AttachmentId)
	if err != nil {
		return nil, err
	}
	return api.CompleteUpload200JSONResponse(apiAttachment(a)), nil
}

func (h *Handlers) GetAttachment(ctx context.Context, req api.GetAttachmentRequestObject) (api.GetAttachmentResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.GetAttachment(ctx, user, req.AttachmentId)
	if err != nil {
		return nil, err
	}
	out := apiAttachment(v.Attachment)
	if v.URLs != nil {
		out.Urls = &api.AttachmentUrls{Original: &v.URLs.Original, Thumb: v.URLs.Thumb, Poster: v.URLs.Poster, ExpiresAt: v.URLs.ExpiresAt}
	}
	return api.GetAttachment200JSONResponse(out), nil
}
