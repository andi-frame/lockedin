package http

import (
	"context"
	"unicode/utf8"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

const maxPayoutNoteRunes = 500

// ledgerCursor is the id of the last line on the previous page.
type ledgerCursor struct {
	Before int64 `json:"b"`
}

func (h *Handlers) ListLedger(ctx context.Context, req api.ListLedgerRequestObject) (api.ListLedgerResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var cur ledgerCursor
	var before *int64
	if ok, err := decodeCursor(req.Params.Cursor, &cur); err != nil {
		return nil, err
	} else if ok {
		before = &cur.Before
	}
	v, err := h.svc.LedgerPage(ctx, user, req.PactId, before, limitOf(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := api.LedgerPage{Items: apiLedgerLines(v.Lines), Balance: v.Balance}
	if v.Next != nil {
		out.NextCursor = encodeCursor(ledgerCursor{Before: *v.Next})
	}
	return api.ListLedger200JSONResponse(out), nil
}

func (h *Handlers) MarkPayoutPaid(ctx context.Context, req api.MarkPayoutPaidRequestObject) (api.MarkPayoutPaidResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var note *string
	if req.Body != nil && req.Body.Note != nil {
		if utf8.RuneCountInString(*req.Body.Note) > maxPayoutNoteRunes {
			return nil, invalid("note must be at most %d characters", maxPayoutNoteRunes)
		}
		note = req.Body.Note
	}
	p, err := h.svc.MarkPayoutPaid(ctx, user, req.PactId, note)
	if err != nil {
		return nil, err
	}
	return api.MarkPayoutPaid200JSONResponse(apiPayout(p)), nil
}

func (h *Handlers) ConfirmPayout(ctx context.Context, req api.ConfirmPayoutRequestObject) (api.ConfirmPayoutResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.ConfirmPayout(ctx, user, req.PactId)
	if err != nil {
		return nil, err
	}
	return api.ConfirmPayout200JSONResponse(apiPayout(p)), nil
}
