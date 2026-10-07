package http

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/http/api"
)

// Limits are requests per Window per client IP (ARCHITECTURE §3). Auth routes and
// upload intents get their own, stricter budgets on top of the global one.
type Limits struct {
	Global, Auth, Upload int
	Window               time.Duration
}

func DefaultLimits() Limits { return Limits{Global: 300, Auth: 10, Upload: 30, Window: time.Minute} }

// opsPath reports probe and scrape endpoints, which are never rate limited or logged loudly.
func opsPath(path string) bool { return path == "/healthz" || path == "/readyz" || path == "/metrics" }

func rateLimiters(rdb *redis.Client, l Limits) []fiber.Handler {
	store := newRedisStorage(rdb, "rl:")
	rule := func(name string, max int, applies func(c fiber.Ctx) bool, code api.ErrorCode) fiber.Handler {
		return limiter.New(limiter.Config{
			Storage:    store,
			Max:        max,
			Expiration: l.Window,
			Next:       func(c fiber.Ctx) bool { return opsPath(c.Path()) || !applies(c) },
			KeyGenerator: func(c fiber.Ctx) string {
				return name + ":" + clientIP(c)
			},
			LimitReached: func(fiber.Ctx) error {
				return problemError(code, "too many requests; wait a moment and try again")
			},
		})
	}
	return []fiber.Handler{
		rule("global", l.Global, func(fiber.Ctx) bool { return true }, api.RateLimited),
		rule("auth", l.Auth, func(c fiber.Ctx) bool { return strings.HasPrefix(c.Path(), basePath+"/auth/") }, api.AuthRateLimited),
		rule("upload", l.Upload, func(c fiber.Ctx) bool {
			return c.Method() == fiber.MethodPost && strings.TrimRight(c.Path(), "/") == basePath+"/uploads"
		}, api.RateLimited),
	}
}
