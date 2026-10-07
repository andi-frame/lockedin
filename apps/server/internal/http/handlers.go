package http

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// Handlers implements the generated strict-server interface. They translate and
// delegate: every rule lives in internal/service or internal/domain (AGENTS.md
// invariant 3), and every pact-scoped call goes through a service function that
// filters by membership and answers 404 to outsiders (invariant 8).
type Handlers struct {
	svc      *service.Service
	auth     *auth.Service
	sessions *auth.Sessions
	secure   bool // Secure cookies; off only for plain-http development
}

func NewHandlers(svc *service.Service, authSvc *auth.Service, secureCookies bool) *Handlers {
	return &Handlers{svc: svc, auth: authSvc, sessions: authSvc.Sessions(), secure: secureCookies}
}

var _ api.StrictServerInterface = (*Handlers)(nil)

// me is the authenticated user. The auth middleware guarantees one on every protected
// route, so a zero id here means a route was left public by mistake.
func me(ctx context.Context) (uuid.UUID, error) {
	id := auth.UserFromContext(ctx)
	if id == uuid.Nil {
		return uuid.Nil, problemError(api.AuthUnauthenticated, "sign in first")
	}
	return id, nil
}

// ---------------------------------------------------------------- auth

// signedIn writes the session cookies and the user. It runs inside the response
// visitor because cookies need the fiber.Ctx that strict handlers do not receive.
type signedIn struct {
	h     *Handlers
	user  api.User
	token string
}

func (s signedIn) write(c fiber.Ctx, status int) error {
	auth.SetSessionCookies(c, s.h.sessions, s.token, s.h.secure)
	c.Status(status)
	return c.JSON(s.user)
}

type registerResponse struct{ signedIn }

func (r registerResponse) VisitRegisterResponse(c fiber.Ctx) error {
	return r.write(c, fiber.StatusCreated)
}

type loginResponse struct{ signedIn }

func (r loginResponse) VisitLoginResponse(c fiber.Ctx) error { return r.write(c, fiber.StatusOK) }

type logoutResponse struct{ h *Handlers }

func (r logoutResponse) VisitLogoutResponse(c fiber.Ctx) error {
	if token := c.Cookies(auth.SessionCookie); token != "" {
		if err := r.h.auth.Logout(c.Context(), token); err != nil {
			return err
		}
	}
	auth.ClearSessionCookies(c, r.h.secure)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handlers) Register(ctx context.Context, req api.RegisterRequestObject) (api.RegisterResponseObject, error) {
	if req.Body == nil {
		return nil, invalid("a JSON body is required")
	}
	b := req.Body
	in := auth.RegisterInput{Email: string(b.Email), Password: b.Password, DisplayName: b.DisplayName}
	if b.Locale != nil {
		in.Locale = string(*b.Locale)
	}
	if b.Timezone != nil {
		in.Timezone = *b.Timezone
	}
	user, err := h.auth.Register(ctx, in)
	if err != nil {
		return nil, err
	}
	token, err := h.sessions.Create(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return registerResponse{signedIn{h: h, user: apiUser(user), token: token}}, nil
}

func (h *Handlers) Login(ctx context.Context, req api.LoginRequestObject) (api.LoginResponseObject, error) {
	if req.Body == nil {
		return nil, invalid("a JSON body is required")
	}
	user, token, err := h.auth.Login(ctx, string(req.Body.Email), req.Body.Password, ClientIPFromContext(ctx))
	if err != nil {
		return nil, err
	}
	return loginResponse{signedIn{h: h, user: apiUser(user), token: token}}, nil
}

func (h *Handlers) Logout(context.Context, api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	return logoutResponse{h: h}, nil
}

func (h *Handlers) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	id, err := me(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.Me(ctx, id)
	if err != nil {
		return nil, err
	}
	return api.GetMe200JSONResponse(apiUser(u)), nil
}

// ---------------------------------------------------------------- today and review queue

func (h *Handlers) GetToday(ctx context.Context, _ api.GetTodayRequestObject) (api.GetTodayResponseObject, error) {
	id, err := me(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.Today(ctx, id)
	if err != nil {
		return nil, err
	}
	out := api.Today{
		ServerTime: v.ServerTime, ReviewQueueCount: int(v.ReviewCount),
		MyCheckIns: make([]api.TodayCheckIn, len(v.CheckIns)), Pacts: make([]api.TodayPact, len(v.Pacts)),
	}
	for i, c := range v.CheckIns {
		out.MyCheckIns[i] = api.TodayCheckIn{
			CheckIn: apiCheckIn(c.CheckIn), PactTitle: c.PactTitle, Member: apiRef(c.Member), WordCount: ptrInt(c.WordCount),
		}
	}
	for i, p := range v.Pacts {
		out.Pacts[i] = api.TodayPact{
			PactId: p.Pact.ID, Title: p.Pact.Title, Status: api.PactStatus(p.Pact.Status), MyRole: api.Role(p.MyRole),
			Balance: p.Balance, Partner: apiRef(p.Partner), NextDeadline: p.NextDeadline, RecentLedger: apiLedgerLines(p.Recent),
		}
	}
	return api.GetToday200JSONResponse(out), nil
}

type reviewCursor struct {
	Deadline time.Time `json:"d"`
	ID       uuid.UUID `json:"i"`
}

func (h *Handlers) GetReviewQueue(ctx context.Context, req api.GetReviewQueueRequestObject) (api.GetReviewQueueResponseObject, error) {
	id, err := me(ctx)
	if err != nil {
		return nil, err
	}
	var cur reviewCursor
	var after *service.ReviewCursor
	if ok, err := decodeCursor(req.Params.Cursor, &cur); err != nil {
		return nil, err
	} else if ok {
		after = &service.ReviewCursor{Deadline: cur.Deadline, ID: cur.ID}
	}
	items, next, err := h.svc.ReviewQueue(ctx, id, after, limitOf(req.Params.Limit))
	if err != nil {
		return nil, err
	}
	out := api.ReviewQueuePage{Items: make([]api.ReviewQueueItem, len(items))}
	for i, it := range items {
		out.Items[i] = api.ReviewQueueItem{
			CheckIn: apiCheckIn(it.CheckIn), PactTitle: it.PactTitle, Member: apiRef(it.Member),
			WordCount: int(it.WordCount), AttachmentCount: int(it.AttachmentCount),
		}
	}
	if next != nil {
		out.NextCursor = encodeCursor(reviewCursor{Deadline: next.Deadline, ID: next.ID})
	}
	return api.GetReviewQueue200JSONResponse(out), nil
}
