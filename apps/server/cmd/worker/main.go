// Command worker runs the background jobs (asynq): settlement sweeps, the outbox relay,
// reminders and email. See internal/jobs for the schedule.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"

	"github.com/andi-frame/lockedin/apps/server/internal/app"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/domain"
	"github.com/andi-frame/lockedin/apps/server/internal/jobs"
	"github.com/andi-frame/lockedin/apps/server/internal/notify"
	"github.com/andi-frame/lockedin/apps/server/internal/service"
	"github.com/andi-frame/lockedin/apps/server/internal/store"
)

func main() {
	os.Exit(app.Main("worker", run))
}

func run(ctx context.Context, cfg config.Config, log *slog.Logger) error {
	pool, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	redisOpt, err := asynq.ParseRedisURI(cfg.RedisURL)
	if err != nil {
		return fmt.Errorf("REDIS_URL: %w", err)
	}

	sender, err := notify.NewSMTP(cfg.SMTPURL, cfg.MailFrom)
	if err != nil {
		return err
	}
	svc := service.New(store.NewStore(pool), domain.SystemClock{})
	opts := jobs.Options{
		Redis:       redisOpt,
		Svc:         svc,
		Mail:        &jobs.Mail{Svc: svc, Renderer: notify.NewRenderer(cfg.BaseURL), Sender: sender},
		Log:         log,
		Concurrency: cfg.WorkerConcurrency,
	}
	if cfg.WorkerMetricsPort > 0 {
		opts.MetricsAddr = fmt.Sprintf(":%d", cfg.WorkerMetricsPort)
	}
	return jobs.Run(ctx, opts)
}
