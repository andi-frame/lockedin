package http

import (
	"context"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

const maxMarkRead = 100

type notificationCursor struct {
	Before int64 `json:"b"`
}

func (h *Handlers) ListNotifications(ctx context.Context, req api.ListNotificationsRequestObject) (api.ListNotificationsResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var cur notificationCursor
	var before *int64
	if ok, err := decodeCursor(req.Params.Cursor, &cur); err != nil {
		return nil, err
	} else if ok {
		before = &cur.Before
	}
	unreadOnly := req.Params.UnreadOnly != nil && *req.Params.UnreadOnly
	rows, next, unread, err := h.svc.ListNotifications(ctx, user, before, limitOf(req.Params.Limit), unreadOnly)
	if err != nil {
		return nil, err
	}
	out := api.NotificationPage{Items: make([]api.Notification, len(rows)), UnreadCount: int(unread)}
	for i, n := range rows {
		if out.Items[i], err = apiNotification(n); err != nil {
			return nil, err
		}
	}
	if next != nil {
		out.NextCursor = encodeCursor(notificationCursor{Before: *next})
	}
	return api.ListNotifications200JSONResponse(out), nil
}

func (h *Handlers) MarkNotificationsRead(ctx context.Context, req api.MarkNotificationsReadRequestObject) (api.MarkNotificationsReadResponseObject, error) {
	user, err := me(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil || len(req.Body.Ids) == 0 || len(req.Body.Ids) > maxMarkRead {
		return nil, invalid("ids must hold 1 to %d notification ids", maxMarkRead)
	}
	if err := h.svc.MarkNotificationsRead(ctx, user, req.Body.Ids); err != nil {
		return nil, err
	}
	return api.MarkNotificationsRead204Response{}, nil
}
