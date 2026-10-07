// Command worker runs background jobs (asynq). Queues arrive in PLAN phase 3.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/andi-frame/lockedin/apps/server/internal/app"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
)

func main() {
	os.Exit(app.Main("worker", func(ctx context.Context, _ config.Config, log *slog.Logger) error {
		log.Info("worker skeleton ready")
		<-ctx.Done()
		return nil
	}))
}
