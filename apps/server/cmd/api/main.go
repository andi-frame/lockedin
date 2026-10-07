// Command api serves the Tepati HTTP API (Fiber).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/app"
	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	httpapi "github.com/andi-frame/lockedin/apps/server/internal/http"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func main() {
	os.Exit(app.Main("api", run))
}

func run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("REDIS_URL: %w", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()

	srv := httpapi.New(httpapi.Deps{
		Config:   cfg,
		Log:      log,
		Redis:    rdb,
		Sessions: auth.NewSessions(rdb, cfg.SessionSecret),
		// Handlers arrive with PLAN 2.3; until then only /healthz, /readyz and /metrics answer.
		Checks: []httpapi.ReadyCheck{httpapi.DatabaseCheck(pool), httpapi.RedisCheck(rdb), httpapi.StorageCheck(cfg.Storage)},
		Limits: httpapi.DefaultLimits(),
	})
	return httpapi.Serve(ctx, srv, cfg.APIPort, log)
}
