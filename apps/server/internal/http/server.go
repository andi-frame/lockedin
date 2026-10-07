// Package http is the Fiber app: the middleware stack from ARCHITECTURE §3, the
// problem+json error handler, the ops endpoints, and the mount point for the
// generated strict-server handlers (PLAN 2.3).
package http

import (
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

const (
	basePath     = "/api/v1"
	maxJSONBytes = 1 << 20 // uploads never pass through the API in presigned mode
)

// Deps is everything the HTTP layer needs. Handlers may be nil while the contract
// is being implemented: the app then serves only the ops endpoints.
type Deps struct {
	Config   config.Config
	Log      *slog.Logger
	Redis    *redis.Client
	Sessions *auth.Sessions
	Handlers api.StrictServerInterface
	Checks   []ReadyCheck
	Limits   Limits
}

// New builds the app. Middleware order (ARCHITECTURE §3):
//
//	requestid → access log (outside recover, so panics are logged as the 500 they
//	become) → recover → cors (dev only) → rate limits → session auth + CSRF →
//	idempotency → body limit (Fiber enforces BodyLimit while reading the request)
//	→ handler
func New(d Deps) *fiber.App {
	if d.Limits.Window == 0 {
		d.Limits = DefaultLimits()
	}
	m := newMetrics()
	app := fiber.New(fiber.Config{
		AppName:      "tepati-api",
		ErrorHandler: errorHandler(d.Log),
		BodyLimit:    maxJSONBytes,
		// Behind Caddy (or the Next dev proxy) the client IP is in X-Forwarded-For,
		// but only trust it from a proxy on a private or loopback address.
		ProxyHeader:      fiber.HeaderXForwardedFor,
		TrustProxy:       true,
		TrustProxyConfig: fiber.TrustProxyConfig{Loopback: true, Private: true},
	})

	app.Use(requestid.New())
	app.Use(func(c fiber.Ctx) error { // handlers and services read the caller's address from the context
		c.SetContext(withClientIP(c.Context(), clientIP(c)))
		return c.Next()
	})
	routes := routeSet{}
	app.Use(observe(d.Log, m, routes))
	app.Use(recover.New())
	if d.Config.Env == "dev" {
		app.Use(corsDev(d.Config.BaseURL))
	}
	for _, rl := range rateLimiters(d.Redis, d.Limits) {
		app.Use(rl)
	}
	app.Use(basePath, authenticate(d.Sessions), jsonBodies(), idempotency(d.Redis, d.Log))

	registerOps(app, d.Checks, m, d.Log)
	if d.Handlers != nil {
		api.RegisterHandlersWithOptions(app, api.NewStrictHandler(d.Handlers, nil), api.FiberServerOptions{BaseURL: basePath})
	}
	routes.fill(app)
	return app
}

func corsDev(origin string) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     []string{origin},
		AllowCredentials: true,
		AllowMethods:     []string{fiber.MethodGet, fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch, fiber.MethodDelete, fiber.MethodOptions},
		AllowHeaders:     []string{fiber.HeaderContentType, auth.CSRFHeader, idempotencyHeader},
		ExposeHeaders:    []string{fiber.HeaderXRequestID, fiber.HeaderRetryAfter, "Idempotent-Replayed"},
		MaxAge:           600,
	})
}

// authenticate requires a session (and, for unsafe methods, the CSRF header) on
// every /api/v1 route except the public ones. A test ties isPublic to the
// `security: []` operations in api/openapi.yaml.
func authenticate(s *auth.Sessions) fiber.Handler {
	guard := auth.RequireUser(s)
	return func(c fiber.Ctx) error {
		if isPublic(c.Method(), c.Path()) {
			return c.Next()
		}
		return guard(c)
	}
}

// jsonBodies makes body handling uniform before the generated handlers see it. They bind
// a body even where the contract says it is optional, and fail on an empty one, so an
// empty body becomes {}. Anything else must be JSON: form and multipart bodies would be
// parsed by Fiber's other binders, and refusing them also means a cross-site <form> can
// never reach a handler (a JSON content type forces a CORS preflight).
func jsonBodies() fiber.Handler {
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodPost, fiber.MethodPut, fiber.MethodPatch:
		default:
			return c.Next()
		}
		if len(c.Body()) == 0 {
			c.Request().Header.SetContentType(fiber.MIMEApplicationJSON)
			c.Request().SetBody([]byte("{}"))
			return c.Next()
		}
		if ct := strings.ToLower(string(c.Request().Header.ContentType())); !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
			return problemError(api.RequestUnsupportedMediaType, "send the body as application/json")
		}
		return c.Next()
	}
}

// isPublic lists the operations the contract marks `security: []`: register, login,
// and the invite preview (the invite token is its credential).
func isPublic(method, path string) bool {
	rest := strings.TrimRight(strings.TrimPrefix(path, basePath), "/")
	switch {
	case method == fiber.MethodPost && (rest == "/auth/register" || rest == "/auth/login"):
		return true
	case method == fiber.MethodGet && strings.HasPrefix(rest, "/invites/") && strings.Count(rest, "/") == 2:
		return true
	}
	return false
}
