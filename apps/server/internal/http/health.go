package http

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ReadyCheck pings one dependency. Names are database, redis, and storage.
type ReadyCheck struct {
	Name string
	Ping func(context.Context) error
}

const readyTimeout = 2 * time.Second

// registerOps mounts the origin-root endpoints. They sit outside /api/v1, are never
// rate limited, and need no session. Caddy must not proxy /metrics to the internet.
func registerOps(app *fiber.App, checks []ReadyCheck, m *metrics, log *slog.Logger) {
	app.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})
	app.Get("/readyz", func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), readyTimeout)
		defer cancel()

		results := make([]string, len(checks))
		var wg sync.WaitGroup
		for i, ch := range checks {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := ch.Ping(ctx); err != nil {
					log.Warn("readiness check failed", "check", ch.Name, "err", err)
					results[i] = "down"
					return
				}
				results[i] = "ok"
			}()
		}
		wg.Wait()

		status, code := "ok", fiber.StatusOK
		out := make(map[string]string, len(checks))
		for i, ch := range checks {
			out[ch.Name] = results[i]
			if results[i] != "ok" {
				status, code = "degraded", fiber.StatusServiceUnavailable
			}
		}
		// Dependency errors stay in the logs; the body only says which one is down.
		return c.Status(code).JSON(fiber.Map{"status": status, "checks": out})
	})
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})))
}
