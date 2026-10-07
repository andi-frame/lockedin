package http

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

const maxProofLinks = 20

// checkInDetail reloads a check-in after a transition so the caller gets the new
// state together with the actions it now allows.
func (h *Handlers) checkInDetail(ctx context.Context, user, id uuid.UUID) (api.CheckInDetail, error) {
	v, err := h.svc.GetCheckInView(ctx, user, id)
	if err != nil {
		return api.CheckInDetail{}, err
	}
	return apiCheckInDetail(v)
}

func (h *Handlers) GetCheckIn(ctx context.Context, req api.GetCheckInRequestObject) (api.GetCheckInResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.GetCheckIn200JSONResponse(out), nil
}

func (h *Handlers) SubmitProof(ctx context.Context, req api.SubmitProofRequestObject) (api.SubmitProofResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil || b.BodyDoc == nil {
		return nil, invalid("body_doc is required")
	}
	doc, err := json.Marshal(b.BodyDoc)
	if err != nil {
		return nil, invalid("body_doc is not valid JSON")
	}
	in := service.ProofInput{BodyDoc: doc}
	if b.Links != nil {
		if len(*b.Links) > maxProofLinks {
			return nil, invalid("at most %d links", maxProofLinks)
		}
		in.Links = *b.Links
	}
	if b.AttachmentIds != nil {
		in.AttachmentIDs = *b.AttachmentIds
	}
	if _, err := h.svc.SubmitProof(ctx, user, req.CheckInId, in); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.SubmitProof200JSONResponse(out), nil
}

func (h *Handlers) DeclareRest(ctx context.Context, req api.DeclareRestRequestObject) (api.DeclareRestResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.DeclareRest(ctx, user, req.CheckInId); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.DeclareRest200JSONResponse(out), nil
}

func (h *Handlers) ApproveCheckIn(ctx context.Context, req api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.Approve(ctx, user, req.CheckInId); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.ApproveCheckIn200JSONResponse(out), nil
}

func (h *Handlers) RejectCheckIn(ctx context.Context, req api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, invalid("a JSON body is required")
	}
	if _, err := h.svc.Reject(ctx, user, req.CheckInId, req.Body.Reason); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.RejectCheckIn200JSONResponse(out), nil
}

func (h *Handlers) OverrideCheckIn(ctx context.Context, req api.OverrideCheckInRequestObject) (api.OverrideCheckInResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, invalid("a JSON body is required")
	}
	if _, err := h.svc.Override(ctx, user, req.CheckInId, req.Body.Reason); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.OverrideCheckIn200JSONResponse(out), nil
}

func (h *Handlers) DisputeCheckIn(ctx context.Context, req api.DisputeCheckInRequestObject) (api.DisputeCheckInResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, invalid("a JSON body is required")
	}
	if _, err := h.svc.Dispute(ctx, user, req.CheckInId, req.Body.Reason); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.DisputeCheckIn200JSONResponse(out), nil
}

func (h *Handlers) ResolveDispute(ctx context.Context, req api.ResolveDisputeRequestObject) (api.ResolveDisputeResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, invalid("a JSON body is required")
	}
	// SPEC §2: the backer's rationale is mandatory for both outcomes. The state machine
	// only enforces it for a dismissal, so uphold is held to it here.
	if !domain.ValidReason(b.Reason) {
		return nil, domain.ErrReasonRequired
	}
	var uphold bool
	switch b.Outcome {
	case api.ResolveDisputeRequestOutcomeUphold:
		uphold = true
	case api.ResolveDisputeRequestOutcomeDismiss:
	default:
		return nil, invalid("outcome must be uphold or dismiss")
	}
	if _, err := h.svc.ResolveDispute(ctx, user, req.CheckInId, uphold, b.Reason); err != nil {
		return nil, err
	}
	out, err := h.checkInDetail(ctx, user, req.CheckInId)
	if err != nil {
		return nil, err
	}
	return api.ResolveDispute200JSONResponse(out), nil
}
