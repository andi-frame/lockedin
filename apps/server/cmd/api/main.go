// Command api serves the Tepati HTTP API (Fiber). Handlers arrive in PLAN phase 2.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/andi-frame/lockedin/apps/server/internal/app"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
)

func main() {
	os.Exit(app.Main("api", func(ctx context.Context, cfg config.Config, log *slog.Logger) error {
		log.Info("api skeleton ready", "port", cfg.APIPort)
		<-ctx.Done()
		return nil
	}))
}
