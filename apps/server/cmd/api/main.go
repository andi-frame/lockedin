// Command api serves the Tepati HTTP API (Fiber).
package main

import (
	"context"
	"fmt"
	"log/slog"
	nethttp "net/http"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/andi-frame/lockedin/apps/server/internal/app"
	"github.com/andi-frame/lockedin/apps/server/internal/auth"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	httpapi "github.com/andi-frame/lockedin/apps/server/internal/http"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/storage"
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

	st := store.NewStore(pool)
	svc := service.New(st, domain.SystemClock{})
	authSvc := auth.NewService(st, rdb, cfg.SessionSecret)

	blobs, err := storage.FromConfig(cfg)
	if err != nil {
		return err
	}
	deps := httpapi.Deps{
		Config:   cfg,
		Log:      log,
		Redis:    rdb,
		Sessions: authSvc.Sessions(),
		// Cookies are Secure everywhere except plain-http local development.
		Handlers: httpapi.NewHandlers(svc, authSvc, cfg.Env != "dev"),
		Checks:   []httpapi.ReadyCheck{httpapi.DatabaseCheck(pool), httpapi.RedisCheck(rdb), httpapi.StorageCheck(cfg.Storage)},
		Limits:   httpapi.DefaultLimits(),
	}
	// Only the fs driver is served by the API; with s3 the browser talks to Garage directly.
	// PLAN 4.2 hands blobs to the upload handlers.
	if h, ok := blobs.(nethttp.Handler); ok {
		deps.Blob = h
		deps.BlobMaxBytes = int(max(cfg.Upload.ImageMaxBytes, cfg.Upload.VideoMaxBytes, cfg.Upload.FileMaxBytes)) + 1<<20
	}
	return httpapi.Serve(ctx, httpapi.New(deps), cfg.APIPort, log)
}
