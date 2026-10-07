package http

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
)

// stub implements only the operations the middleware tests exercise. Calling any
// other operation panics on the nil embedded interface, which the recover
// middleware turns into a 500.
type stub struct {
	api.StrictServerInterface
	approve func(context.Context, api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error)
	reject  func(context.Context, api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error)
	markRd  func(context.Context, api.MarkNotificationsReadRequestObject) (api.MarkNotificationsReadResponseObject, error)
}

func (s *stub) GetMe(ctx context.Context, _ api.GetMeRequestObject) (api.GetMeResponseObject, error) {
	return api.GetMe200JSONResponse(api.User{Id: auth.UserFromContext(ctx), Email: "sari@example.id"}), nil
}

func (s *stub) Login(context.Context, api.LoginRequestObject) (api.LoginResponseObject, error) {
	return api.Login200JSONResponse(api.User{DisplayName: "Sari", Email: "sari@example.id"}), nil
}

func (s *stub) PreviewInvite(context.Context, api.PreviewInviteRequestObject) (api.PreviewInviteResponseObject, error) {
	return api.PreviewInvite200JSONResponse(api.InvitePreview{PactTitle: "Kalkulus"}), nil
}

func (s *stub) CreateUpload(context.Context, api.CreateUploadRequestObject) (api.CreateUploadResponseObject, error) {
	return api.CreateUpload201JSONResponse(api.UploadIntent{PutUrl: "http://s3.test/put"}), nil
}

func (s *stub) ApproveCheckIn(ctx context.Context, r api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
	return s.approve(ctx, r)
}

func (s *stub) RejectCheckIn(ctx context.Context, r api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error) {
	return s.reject(ctx, r)
}

func (s *stub) MarkNotificationsRead(ctx context.Context, r api.MarkNotificationsReadRequestObject) (api.MarkNotificationsReadResponseObject, error) {
	return s.markRd(ctx, r)
}

// syncBuffer lets a real server goroutine log while the test reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type env struct {
	app      *fiber.App
	mr       *miniredis.Miniredis
	rdb      *redis.Client
	sessions *auth.Sessions
	logs     *syncBuffer
	h        *stub
	calls    atomic.Int32
}

type option func(*Deps)

func withLimits(l Limits) option                      { return func(d *Deps) { d.Limits = l } }
func withChecks(c ...ReadyCheck) option               { return func(d *Deps) { d.Checks = c } }
func withEnv(name string) option                      { return func(d *Deps) { d.Config.Env = name } }
func withHandlers(s api.StrictServerInterface) option { return func(d *Deps) { d.Handlers = s } }

func newEnv(t *testing.T, opts ...option) *env {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	e := &env{mr: mr, rdb: rdb, sessions: auth.NewSessions(rdb, "test-secret"), logs: &syncBuffer{}}
	e.h = &stub{
		approve: func(context.Context, api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
			e.calls.Add(1)
			return api.ApproveCheckIn200JSONResponse(api.CheckInDetail{PactTitle: "approved"}), nil
		},
		reject: func(_ context.Context, r api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error) {
			e.calls.Add(1)
			return api.RejectCheckIn200JSONResponse(api.CheckInDetail{PactTitle: r.Body.Reason}), nil
		},
		markRd: func(context.Context, api.MarkNotificationsReadRequestObject) (api.MarkNotificationsReadResponseObject, error) {
			panic("boom: secret internal detail")
		},
	}
	d := Deps{
		Config:   config.Config{Env: "dev", BaseURL: "http://localhost:3000"},
		Log:      slog.New(slog.NewJSONHandler(e.logs, nil)),
		Redis:    rdb,
		Sessions: e.sessions,
		Handlers: e.h,
		Limits:   Limits{Global: 1000, Auth: 1000, Upload: 1000, Window: time.Minute},
	}
	for _, o := range opts {
		o(&d)
	}
	e.app = New(d)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("app logs:\n%s", e.logs.String())
		}
	})
	return e
}

type login struct{ cookie, csrf string }

func (e *env) loginAs(t *testing.T, user uuid.UUID) login {
	t.Helper()
	tok, err := e.sessions.Create(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	return login{cookie: auth.SessionCookie + "=" + tok, csrf: e.sessions.CSRFToken(tok)}
}

type reply struct {
	status int
	header map[string][]string
	body   []byte
}

func (r reply) get(h string) string {
	for k, v := range r.header {
		if strings.EqualFold(k, h) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}

func (r reply) problem(t *testing.T) api.Problem {
	t.Helper()
	if ct := r.get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("content type = %q, want problem+json (status %d, body %s)", ct, r.status, r.body)
	}
	var p api.Problem
	if err := json.Unmarshal(r.body, &p); err != nil {
		t.Fatalf("problem body: %v (%s)", err, r.body)
	}
	if p.Status != r.status {
		t.Errorf("problem.status = %d but HTTP status = %d", p.Status, r.status)
	}
	return p
}

type req struct {
	method, path, body string
	as                 *login
	idemKey            string
	origin             string
}

func (e *env) do(t *testing.T, r req) reply {
	t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	hr := httptest.NewRequest(r.method, r.path, body)
	if r.body != "" {
		hr.Header.Set("Content-Type", "application/json")
	}
	if r.as != nil {
		hr.Header.Set("Cookie", r.as.cookie)
		if r.method != "GET" && r.method != "HEAD" {
			hr.Header.Set(auth.CSRFHeader, r.as.csrf)
		}
	}
	if r.idemKey != "" {
		hr.Header.Set("Idempotency-Key", r.idemKey)
	}
	if r.origin != "" {
		hr.Header.Set("Origin", r.origin)
	}
	resp, err := e.app.Test(hr, fiber.TestConfig{Timeout: 5 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatalf("%s %s: %v", r.method, r.path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return reply{status: resp.StatusCode, header: resp.Header, body: b}
}

const approvePath = "/api/v1/check-ins/0197a000-0000-7000-8000-000000000001/approve"

// ---------------------------------------------------------------- auth gate

func TestProtectedRouteNeedsSession(t *testing.T) {
	e := newEnv(t)
	r := e.do(t, req{method: "GET", path: "/api/v1/me"})
	if r.status != 401 {
		t.Fatalf("status = %d, want 401", r.status)
	}
	p := r.problem(t)
	if p.Code != api.AuthUnauthenticated {
		t.Errorf("code = %q", p.Code)
	}
	if id := r.get("X-Request-Id"); id == "" || p.RequestId == nil || *p.RequestId != id {
		t.Errorf("request id header %q and body %v should match and be set", id, p.RequestId)
	}
}

func TestSessionReachesHandlerWithUserID(t *testing.T) {
	e := newEnv(t)
	user := uuid.New()
	l := e.loginAs(t, user)
	r := e.do(t, req{method: "GET", path: "/api/v1/me", as: &l})
	if r.status != 200 {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
	var u api.User
	_ = json.Unmarshal(r.body, &u)
	if u.Id != user {
		t.Errorf("handler saw user %v, want %v", u.Id, user)
	}
}

func TestUnsafeMethodNeedsCSRFToken(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	noCSRF := login{cookie: l.cookie}
	r := e.do(t, req{method: "POST", path: approvePath, as: &noCSRF})
	if r.status != 403 || r.problem(t).Code != api.AuthCsrf {
		t.Fatalf("status = %d body %s, want 403 auth.csrf", r.status, r.body)
	}
	hr := httptest.NewRequest("POST", approvePath, nil)
	hr.Header.Set("Cookie", l.cookie)
	hr.Header.Set(auth.CSRFHeader, "forged")
	resp, _ := e.app.Test(hr)
	if resp.StatusCode != 403 {
		t.Errorf("forged csrf token: status = %d, want 403", resp.StatusCode)
	}
	if e.calls.Load() != 0 {
		t.Error("handler ran without a valid CSRF token")
	}
}

func TestPublicRoutesNeedNoSession(t *testing.T) {
	e := newEnv(t)
	if r := e.do(t, req{method: "POST", path: "/api/v1/auth/login", body: `{"email":"a@b.id","password":"x"}`}); r.status != 200 {
		t.Errorf("login: status = %d body %s", r.status, r.body)
	}
	if r := e.do(t, req{method: "GET", path: "/api/v1/invites/abcdefghijklmnopqrstuv"}); r.status != 200 {
		t.Errorf("invite preview: status = %d body %s", r.status, r.body)
	}
}

// ---------------------------------------------------------------- problem+json

func TestDomainAndServiceErrorsBecomeProblems(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   api.ErrorCode
	}{
		{"not found", service.ErrNotFound, 404, api.NotFound},
		{"wrapped domain error", fmt.Errorf("reject: %w", domain.ErrReasonRequired), 400, api.CheckinReasonRequired},
		{"conflict", service.ErrPactState, 409, api.PactInvalidState},
		{"forbidden power", service.ErrNotBacker, 403, api.PactNotBacker},
		{"deadline", domain.ErrDeadlinePassed, 422, api.CheckinDeadlinePassed},
		{"lost race", service.ErrLostRace, 409, api.CheckinConflict},
		{"invite", service.ErrInviteInvalid, 410, api.PactInviteInvalid},
		{"auth rate limit", auth.ErrRateLimited, 429, api.AuthRateLimited},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.h.reject = func(context.Context, api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error) {
				return nil, c.err
			}
			l := e.loginAs(t, uuid.New())
			r := e.do(t, req{method: "POST", path: strings.Replace(approvePath, "approve", "reject", 1), body: `{"reason":"belum cukup bukti"}`, as: &l})
			if r.status != c.status {
				t.Fatalf("status = %d, want %d (body %s)", r.status, c.status, r.body)
			}
			if p := r.problem(t); p.Code != c.code {
				t.Errorf("code = %q, want %q", p.Code, c.code)
			}
		})
	}
}

func TestUnknownErrorIs500AndDoesNotLeak(t *testing.T) {
	e := newEnv(t)
	e.h.approve = func(context.Context, api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
		return nil, errors.New("pq: password authentication failed for user tepati")
	}
	l := e.loginAs(t, uuid.New())
	r := e.do(t, req{method: "POST", path: approvePath, as: &l})
	if r.status != 500 || r.problem(t).Code != api.ServerInternal {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
	if strings.Contains(string(r.body), "password") || strings.Contains(string(r.body), "pq:") {
		t.Errorf("response leaks the internal error: %s", r.body)
	}
	if !strings.Contains(e.logs.String(), "password authentication failed") {
		t.Error("the real error should still be logged for operators")
	}
}

func TestPanicIsRecoveredAsProblem(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	r := e.do(t, req{method: "POST", path: "/api/v1/notifications/read", body: `{"ids":[1]}`, as: &l})
	if r.status != 500 || r.problem(t).Code != api.ServerInternal {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
	if strings.Contains(string(r.body), "secret") {
		t.Errorf("panic value leaked: %s", r.body)
	}
	if again := e.do(t, req{method: "GET", path: "/api/v1/me", as: &l}); again.status != 200 {
		t.Errorf("server did not survive the panic: status %d", again.status)
	}
}

func TestRoutingErrorsAreProblems(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	if r := e.do(t, req{method: "GET", path: "/api/v1/nope", as: &l}); r.status != 404 || r.problem(t).Code != api.NotFound {
		t.Errorf("unknown route: status %d body %s", r.status, r.body)
	}
	if r := e.do(t, req{method: "DELETE", path: "/api/v1/me", as: &l}); r.status != 405 || r.problem(t).Code != api.MethodNotAllowed {
		t.Errorf("wrong method: status %d body %s", r.status, r.body)
	}
}

func TestBodyOverOneMegabyteIsRejected(t *testing.T) {
	// app.Test cannot show what a client sees for an oversize body, and streaming 2 MB
	// at a server that answers early races with the connection closing. So declare the
	// size in the headers over a real socket and read the reply.
	e := newEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = e.app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = e.app.Shutdown() })

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	lines := []string{
		"POST /api/v1/auth/login HTTP/1.1",
		"Host: tepati.test",
		"Content-Type: application/json",
		fmt.Sprintf("Content-Length: %d", 2<<20),
		"", "",
	}
	_, _ = io.WriteString(conn, strings.Join(lines, "\r\n"))
	resp, err := nethttp.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("a client should get a response, not a dropped connection: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 413 {
		t.Fatalf("status = %d body %.200s", resp.StatusCode, raw)
	}
	var p api.Problem
	if err := json.Unmarshal(raw, &p); err != nil || p.Code != api.RequestTooLarge {
		t.Errorf("body = %.200s", raw)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/problem+json") {
		t.Errorf("content type = %q", resp.Header.Get("Content-Type"))
	}
}

func TestMalformedJSONIsValidationFailed(t *testing.T) {
	e := newEnv(t)
	r := e.do(t, req{method: "POST", path: "/api/v1/auth/login", body: `{"email":`})
	if r.status != 400 || r.problem(t).Code != api.ValidationFailed {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
}

// ---------------------------------------------------------------- rate limits

func TestGlobalRateLimitReturns429(t *testing.T) {
	e := newEnv(t, withLimits(Limits{Global: 3, Auth: 100, Upload: 100, Window: time.Minute}))
	l := e.loginAs(t, uuid.New())
	for i := 1; i <= 3; i++ {
		if r := e.do(t, req{method: "GET", path: "/api/v1/me", as: &l}); r.status != 200 {
			t.Fatalf("request %d: status = %d", i, r.status)
		}
	}
	r := e.do(t, req{method: "GET", path: "/api/v1/me", as: &l})
	if r.status != 429 || r.problem(t).Code != api.RateLimited {
		t.Fatalf("status = %d body %s, want 429 rate_limited", r.status, r.body)
	}
	if r.get("Retry-After") == "" {
		t.Error("429 should carry Retry-After")
	}
	for i := 0; i < 10; i++ {
		if h := e.do(t, req{method: "GET", path: "/healthz"}); h.status != 200 {
			t.Fatalf("health probes must never be rate limited, got %d", h.status)
		}
	}
}

func TestAuthRoutesHaveAStricterLimit(t *testing.T) {
	e := newEnv(t, withLimits(Limits{Global: 100, Auth: 2, Upload: 100, Window: time.Minute}))
	body := `{"email":"a@b.id","password":"x"}`
	for i := 1; i <= 2; i++ {
		if r := e.do(t, req{method: "POST", path: "/api/v1/auth/login", body: body}); r.status != 200 {
			t.Fatalf("login %d: status = %d", i, r.status)
		}
	}
	r := e.do(t, req{method: "POST", path: "/api/v1/auth/login", body: body})
	if r.status != 429 || r.problem(t).Code != api.AuthRateLimited {
		t.Fatalf("status = %d body %s, want 429 auth.rate_limited", r.status, r.body)
	}
	l := e.loginAs(t, uuid.New())
	if other := e.do(t, req{method: "GET", path: "/api/v1/me", as: &l}); other.status != 200 {
		t.Errorf("non-auth routes must not share the auth budget: status %d", other.status)
	}
}

func TestUploadIntentHasItsOwnLimit(t *testing.T) {
	e := newEnv(t, withLimits(Limits{Global: 100, Auth: 100, Upload: 2, Window: time.Minute}))
	l := e.loginAs(t, uuid.New())
	body := `{"pact_id":"0197a000-0000-7000-8000-000000000001","kind":"image","mime":"image/webp","bytes":10}`
	for i := 1; i <= 2; i++ {
		if r := e.do(t, req{method: "POST", path: "/api/v1/uploads", body: body, as: &l}); r.status != 201 {
			t.Fatalf("intent %d: status = %d body %s", i, r.status, r.body)
		}
	}
	if r := e.do(t, req{method: "POST", path: "/api/v1/uploads", body: body, as: &l}); r.status != 429 {
		t.Fatalf("third intent: status = %d, want 429", r.status)
	}
}

// ---------------------------------------------------------------- idempotency

func TestIdempotencyReplaysTheStoredResponse(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	first := e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"})
	second := e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"})
	if first.status != 200 || second.status != 200 {
		t.Fatalf("statuses = %d, %d", first.status, second.status)
	}
	if e.calls.Load() != 1 {
		t.Errorf("handler ran %d times, want 1", e.calls.Load())
	}
	if !bytes.Equal(first.body, second.body) {
		t.Errorf("replayed body differs:\n%s\n%s", first.body, second.body)
	}
	if second.get("Idempotent-Replayed") != "true" || first.get("Idempotent-Replayed") != "" {
		t.Errorf("replay marker: first=%q second=%q", first.get("Idempotent-Replayed"), second.get("Idempotent-Replayed"))
	}
	if ct := second.get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("replayed content type = %q", ct)
	}
}

func TestIdempotencyKeyReusedWithDifferentBodyIs422(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	path := strings.Replace(approvePath, "approve", "reject", 1)
	if r := e.do(t, req{method: "POST", path: path, body: `{"reason":"alasan pertama ya"}`, as: &l, idemKey: "key-12345678"}); r.status != 200 {
		t.Fatalf("first: %d %s", r.status, r.body)
	}
	r := e.do(t, req{method: "POST", path: path, body: `{"reason":"alasan lain sama sekali"}`, as: &l, idemKey: "key-12345678"})
	if r.status != 422 || r.problem(t).Code != api.IdempotencyKeyReused {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
	if e.calls.Load() != 1 {
		t.Errorf("handler ran %d times, want 1", e.calls.Load())
	}
}

func TestIdempotencyIsScopedToUserAndRoute(t *testing.T) {
	e := newEnv(t)
	a, b := e.loginAs(t, uuid.New()), e.loginAs(t, uuid.New())
	e.do(t, req{method: "POST", path: approvePath, as: &a, idemKey: "key-12345678"})
	e.do(t, req{method: "POST", path: approvePath, as: &b, idemKey: "key-12345678"})
	other := strings.Replace(approvePath, "000000000001", "000000000002", 1)
	e.do(t, req{method: "POST", path: other, as: &a, idemKey: "key-12345678"})
	if e.calls.Load() != 3 {
		t.Errorf("handler ran %d times, want 3 (another user or another check-in must not replay)", e.calls.Load())
	}
}

func TestIdempotencyDoesNotCacheFailures(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	fail := true
	e.h.approve = func(context.Context, api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
		e.calls.Add(1)
		if fail {
			return nil, service.ErrLostRace
		}
		return api.ApproveCheckIn200JSONResponse(api.CheckInDetail{}), nil
	}
	if r := e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"}); r.status != 409 {
		t.Fatalf("first: %d", r.status)
	}
	fail = false
	if r := e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"}); r.status != 200 {
		t.Fatalf("retry after a failure must run again, got %d %s", r.status, r.body)
	}
	if e.calls.Load() != 2 {
		t.Errorf("handler ran %d times, want 2", e.calls.Load())
	}
}

func TestIdempotencyRejectsAConcurrentDuplicate(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	started, release := make(chan struct{}), make(chan struct{})
	e.h.approve = func(context.Context, api.ApproveCheckInRequestObject) (api.ApproveCheckInResponseObject, error) {
		e.calls.Add(1)
		close(started)
		<-release
		return api.ApproveCheckIn200JSONResponse(api.CheckInDetail{}), nil
	}
	done := make(chan reply, 1)
	go func() { done <- e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"}) }()
	<-started
	dup := e.do(t, req{method: "POST", path: approvePath, as: &l, idemKey: "key-12345678"})
	if dup.status != 409 || dup.problem(t).Code != api.IdempotencyInProgress {
		t.Errorf("duplicate while in flight: status %d body %s", dup.status, dup.body)
	}
	close(release)
	if first := <-done; first.status != 200 {
		t.Errorf("first request: status %d", first.status)
	}
	if e.calls.Load() != 1 {
		t.Errorf("handler ran %d times, want 1", e.calls.Load())
	}
}

func TestWithoutAKeyEveryRequestRuns(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	e.do(t, req{method: "POST", path: approvePath, as: &l})
	e.do(t, req{method: "POST", path: approvePath, as: &l})
	if e.calls.Load() != 2 {
		t.Errorf("handler ran %d times, want 2", e.calls.Load())
	}
}

// ---------------------------------------------------------------- ops endpoints

func TestHealthzIsAlwaysOK(t *testing.T) {
	e := newEnv(t, withChecks(ReadyCheck{Name: "database", Ping: func(context.Context) error { return errors.New("down") }}))
	r := e.do(t, req{method: "GET", path: "/healthz"})
	if r.status != 200 || !strings.Contains(string(r.body), `"ok"`) {
		t.Fatalf("status = %d body %s", r.status, r.body)
	}
}

func TestReadyzReportsEachDependency(t *testing.T) {
	ok := func(context.Context) error { return nil }
	bad := func(context.Context) error { return errors.New("connection refused to 10.0.0.5") }

	e := newEnv(t, withChecks(ReadyCheck{"database", ok}, ReadyCheck{"redis", ok}, ReadyCheck{"storage", ok}))
	r := e.do(t, req{method: "GET", path: "/readyz"})
	if r.status != 200 {
		t.Fatalf("all up: status = %d body %s", r.status, r.body)
	}

	e = newEnv(t, withChecks(ReadyCheck{"database", ok}, ReadyCheck{"redis", bad}, ReadyCheck{"storage", ok}))
	r = e.do(t, req{method: "GET", path: "/readyz"})
	if r.status != 503 {
		t.Fatalf("redis down: status = %d body %s", r.status, r.body)
	}
	var got struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.Unmarshal(r.body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "degraded" || got.Checks["redis"] != "down" || got.Checks["database"] != "ok" || got.Checks["storage"] != "ok" {
		t.Errorf("readiness = %+v", got)
	}
	if strings.Contains(string(r.body), "10.0.0.5") {
		t.Errorf("readiness leaks the dependency error: %s", r.body)
	}
}

func TestMetricsExposeRequestLatencyByRoutePattern(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	e.do(t, req{method: "POST", path: approvePath, as: &l})
	r := e.do(t, req{method: "GET", path: "/metrics"})
	body := string(r.body)
	if r.status != 200 || !strings.Contains(body, "tepati_http_request_duration_seconds_bucket") {
		t.Fatalf("status = %d, body %.300s", r.status, body)
	}
	if !strings.Contains(body, `route="/api/v1/check-ins/:checkInId/approve"`) {
		t.Errorf("metrics should label by route pattern, not by raw path (ids would explode cardinality):\n%s", body)
	}
	if strings.Contains(body, "0197a000-0000-7000-8000-000000000001") {
		t.Error("a raw id leaked into a metric label")
	}
}

// ---------------------------------------------------------------- logging, CORS

func TestAccessLogHasContextAndNoBodies(t *testing.T) {
	e := newEnv(t)
	user := uuid.New()
	l := e.loginAs(t, user)
	e.do(t, req{method: "GET", path: "/api/v1/me", as: &l})
	e.do(t, req{method: "POST", path: "/api/v1/auth/login", body: `{"email":"a@b.id","password":"hunter2-secret"}`})

	var lines []map[string]any
	for _, ln := range strings.Split(strings.TrimSpace(e.logs.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil {
			t.Fatalf("log line is not JSON: %q", ln)
		}
		if m["msg"] == "request" {
			lines = append(lines, m)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("got %d access log lines, want 2:\n%s", len(lines), e.logs.String())
	}
	me := lines[0]
	if me["request_id"] == "" || me["method"] != "GET" || me["status"] != float64(200) || me["user_id"] != user.String() {
		t.Errorf("access log line = %v", me)
	}
	if _, ok := me["duration_ms"]; !ok {
		t.Errorf("access log should record duration_ms: %v", me)
	}
	if strings.Contains(e.logs.String(), "hunter2-secret") || strings.Contains(e.logs.String(), l.cookie[len(auth.SessionCookie)+1:]) {
		t.Error("logs must never contain request bodies or session tokens")
	}
}

func TestCORSIsDevOnly(t *testing.T) {
	dev := newEnv(t, withEnv("dev"))
	pre := httptest.NewRequest("OPTIONS", "/api/v1/me", nil)
	pre.Header.Set("Origin", "http://localhost:3000")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	pre.Header.Set("Access-Control-Request-Headers", "content-type,x-csrf-token,idempotency-key")
	resp, err := dev.app.Test(pre)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 204 || resp.Header.Get("Access-Control-Allow-Origin") != "http://localhost:3000" || resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("dev preflight: status %d headers %v (a preflight must pass before auth)", resp.StatusCode, resp.Header)
	}
	if r := dev.do(t, req{method: "GET", path: "/healthz", origin: "http://localhost:3000"}); r.get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Errorf("dev simple request headers = %v", r.header)
	}

	prod := newEnv(t, withEnv("production"))
	if r := prod.do(t, req{method: "GET", path: "/healthz", origin: "http://localhost:3000"}); r.get("Access-Control-Allow-Origin") != "" {
		t.Errorf("production must stay same-origin, got ACAO %q", r.get("Access-Control-Allow-Origin"))
	}
}

func TestServerWithoutHandlersStillServesOps(t *testing.T) {
	e := newEnv(t, withHandlers(nil))
	if r := e.do(t, req{method: "GET", path: "/healthz"}); r.status != 200 {
		t.Errorf("healthz status = %d", r.status)
	}
}

// Fiber's c.IP() is empty when a trusted-proxy header is configured but the request
// has none, which would put every client into one shared rate-limit bucket.
func TestClientIPIsNeverEmpty(t *testing.T) {
	e := newEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = e.app.Listener(ln, fiber.ListenConfig{DisableStartupMessage: true}) }()
	t.Cleanup(func() { _ = e.app.Shutdown() })

	resp, err := nethttp.Get("http://" + ln.Addr().String() + "/api/v1/me")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.Contains(e.logs.String(), `"ip":"127.0.0.1"`) {
		t.Errorf("the access log should carry the peer address, got: %s", e.logs.String())
	}
	// A forwarded address wins when it comes from a trusted (loopback/private) proxy.
	hr, _ := nethttp.NewRequest("GET", "http://"+ln.Addr().String()+"/api/v1/me", nil)
	hr.Header.Set("X-Forwarded-For", "203.0.113.9")
	resp, err = nethttp.DefaultClient.Do(hr)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !strings.Contains(e.logs.String(), `"ip":"203.0.113.9"`) {
		t.Errorf("X-Forwarded-For from a loopback proxy should be used, got: %s", e.logs.String())
	}
}

// ---------------------------------------------------------------- request bodies

// The generated handlers bind a body even when the contract marks it optional, and
// reject an empty one. An empty body must mean {}.
func TestEmptyBodyIsAnEmptyJSONObject(t *testing.T) {
	e := newEnv(t)
	var gotBody bool
	var gotReason string
	e.h.reject = func(_ context.Context, r api.RejectCheckInRequestObject) (api.RejectCheckInResponseObject, error) {
		gotBody, gotReason = r.Body != nil, r.Body.Reason
		return api.RejectCheckIn200JSONResponse(api.CheckInDetail{}), nil
	}
	l := e.loginAs(t, uuid.New())
	r := e.do(t, req{method: "POST", path: strings.Replace(approvePath, "approve", "reject", 1), as: &l})
	if r.status != 200 || !gotBody || gotReason != "" {
		t.Fatalf("status %d body %s (handler saw body=%v reason=%q)", r.status, r.body, gotBody, gotReason)
	}
}

func TestNonJSONBodiesAreRefused(t *testing.T) {
	e := newEnv(t)
	l := e.loginAs(t, uuid.New())
	hr := httptest.NewRequest("POST", strings.Replace(approvePath, "approve", "reject", 1), strings.NewReader("reason=belum+cukup+ya"))
	hr.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	hr.Header.Set("Cookie", l.cookie)
	hr.Header.Set(auth.CSRFHeader, l.csrf)
	resp, err := e.app.Test(hr)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 415 || !strings.Contains(string(raw), string(api.RequestUnsupportedMediaType)) {
		t.Fatalf("status = %d body %s, want 415 request.unsupported_media_type", resp.StatusCode, raw)
	}
	if e.calls.Load() != 0 {
		t.Error("the handler must not run for a form body")
	}
}
