package http

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

const (
	maxTitleRunes       = 120
	maxDescriptionRunes = 1000
	maxSignatureRunes   = 80
)

// pactView reloads a pact after a mutation so every pact response has the same shape.
func (h *Handlers) pactView(ctx context.Context, user, pactID uuid.UUID) (api.Pact, error) {
	v, err := h.svc.GetPactView(ctx, user, pactID)
	if err != nil {
		return api.Pact{}, err
	}
	return apiPact(v)
}

func draftInput(b *api.PactDraft) (service.DraftInput, error) {
	if b == nil {
		return service.DraftInput{}, invalid("a JSON body is required")
	}
	title := strings.TrimSpace(b.Title)
	if n := utf8.RuneCountInString(title); n < 1 || n > maxTitleRunes {
		return service.DraftInput{}, invalid("title must be 1 to %d characters", maxTitleRunes)
	}
	if b.Description != nil && utf8.RuneCountInString(*b.Description) > maxDescriptionRunes {
		return service.DraftInput{}, invalid("description must be at most %d characters", maxDescriptionRunes)
	}
	terms, err := termsFromAPI(b.Terms)
	if err != nil {
		return service.DraftInput{}, err
	}
	return service.DraftInput{Title: title, Description: b.Description, Terms: terms}, nil
}

type pactCursor struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

func (h *Handlers) ListPacts(ctx context.Context, req api.ListPactsRequestObject) (api.ListPactsResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var cur pactCursor
	var after *service.PactCursor
	if ok, err := decodeCursor(req.Params.Cursor, &cur); err != nil {
		return nil, err
	} else if ok {
		after = &service.PactCursor{At: cur.At, ID: cur.ID}
	}
	views, next, err := h.svc.ListPactViews(ctx, user, after, limitOf(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := api.PactPage{Items: make([]api.Pact, len(views))}
	for i, v := range views {
		if out.Items[i], err = apiPact(v); err != nil {
			return nil, err
		}
	}
	if next != nil {
		out.NextCursor = encodeCursor(pactCursor{At: next.At, ID: next.ID})
	}
	return api.ListPacts200JSONResponse(out), nil
}

func (h *Handlers) CreatePact(ctx context.Context, req api.CreatePactRequestObject) (api.CreatePactResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	in, err := draftInput(req.Body)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.CreateDraft(ctx, user, in)
	if err != nil {
		return nil, err
	}
	out, err := h.pactView(ctx, user, p.ID)
	if err != nil {
		return nil, err
	}
	return api.CreatePact201JSONResponse(out), nil
}

func (h *Handlers) GetPact(ctx context.Context, req api.GetPactRequestObject) (api.GetPactResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	out, err := h.pactView(ctx, user, req.PactId)
	if err != nil {
		return nil, err
	}
	return api.GetPact200JSONResponse(out), nil
}

func (h *Handlers) UpdatePact(ctx context.Context, req api.UpdatePactRequestObject) (api.UpdatePactResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	in, err := draftInput(req.Body)
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.UpdateTerms(ctx, user, req.PactId, in); err != nil {
		return nil, err
	}
	out, err := h.pactView(ctx, user, req.PactId)
	if err != nil {
		return nil, err
	}
	return api.UpdatePact200JSONResponse(out), nil
}

func (h *Handlers) ProposePact(ctx context.Context, req api.ProposePactRequestObject) (api.ProposePactResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var email *string
	if req.Body != nil && req.Body.Email != nil {
		e := string(*req.Body.Email)
		email = &e
	}
	token, err := h.svc.Propose(ctx, user, req.PactId, email)
	if err != nil {
		return nil, err
	}
	return api.ProposePact200JSONResponse(api.Proposal{InviteToken: token}), nil
}

func (h *Handlers) AcceptPact(ctx context.Context, req api.AcceptPactRequestObject) (api.AcceptPactResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if b == nil {
		return nil, invalid("a JSON body is required")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(b.SignatureName)); n < 1 || n > maxSignatureRunes {
		return nil, invalid("signature_name must be 1 to %d characters", maxSignatureRunes)
	}
	if len(b.TermsHash) != 64 {
		return nil, invalid("terms_hash must be a 64-character hex digest")
	}
	if _, err := h.svc.Accept(ctx, user, req.PactId, b.TermsHash, b.SignatureName); err != nil {
		return nil, err
	}
	out, err := h.pactView(ctx, user, req.PactId)
	if err != nil {
		return nil, err
	}
	return api.AcceptPact200JSONResponse(out), nil
}

func (h *Handlers) PreviewInvite(ctx context.Context, req api.PreviewInviteRequestObject) (api.PreviewInviteResponseObject, error) {
	v, err := h.svc.InvitePreview(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	terms, err := apiTerms(v.Terms)
	if err != nil {
		return nil, err
	}
	return api.PreviewInvite200JSONResponse(api.InvitePreview{
		PactTitle: v.Pact.Title, Inviter: apiRef(v.Inviter), Terms: terms, TermsHash: v.Pact.TermsHash,
		Status: api.PactStatus(v.Pact.Status), DoerSlotOpen: v.DoerSlotOpen,
	}), nil
}

func (h *Handlers) JoinInvite(ctx context.Context, req api.JoinInviteRequestObject) (api.JoinInviteResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.JoinByInvite(ctx, user, req.Token)
	if err != nil {
		return nil, err
	}
	out, err := h.pactView(ctx, user, p.ID)
	if err != nil {
		return nil, err
	}
	return api.JoinInvite200JSONResponse(out), nil
}

func (h *Handlers) ListPactCheckIns(ctx context.Context, req api.ListPactCheckInsRequestObject) (api.ListPactCheckInsResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var from, to *time.Time
	if req.Params.From != nil {
		from = &req.Params.From.Time
	}
	if req.Params.To != nil {
		to = &req.Params.To.Time
	}
	if from != nil && to != nil && to.Before(*from) {
		return nil, invalid("to must not be before from")
	}
	rows, err := h.svc.ListPactCheckIns(ctx, user, req.PactId, from, to)
	if err != nil {
		return nil, err
	}
	out := api.CheckInList{Items: make([]api.CheckIn, len(rows))}
	for i, r := range rows {
		out.Items[i] = apiCheckIn(r)
	}
	return api.ListPactCheckIns200JSONResponse(out), nil
}
