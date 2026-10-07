// Package app holds the boot sequence shared by the api and worker binaries.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/andi-frame/lockedin/apps/server/internal/buildinfo"
	"github.com/andi-frame/lockedin/apps/server/internal/config"
	"github.com/andi-frame/lockedin/apps/server/internal/logging"
)

// Main loads config, builds the logger, and runs fn until SIGINT/SIGTERM.
// It returns the process exit code.
func Main(service string, fn func(ctx context.Context, cfg config.Config, log *slog.Logger) error) int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	log := logging.New(service, cfg.Env)
	log.Info("starting", "version", buildinfo.Version, "env", cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := fn(ctx, cfg, log); err != nil {
		log.Error("stopped with error", "err", err)
		return 1
	}
	log.Info("stopped")
	return 0
}
