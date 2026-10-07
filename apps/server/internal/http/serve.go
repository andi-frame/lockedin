package http

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
)

const shutdownTimeout = 10 * time.Second

// Serve listens until ctx is cancelled, then drains in-flight requests.
func Serve(ctx context.Context, app *fiber.App, port int, log *slog.Logger) error {
	errc := make(chan error, 1)
	go func() {
		errc <- app.Listen(fmt.Sprintf(":%d", port), fiber.ListenConfig{DisableStartupMessage: true})
	}()
	log.Info("api listening", "port", port)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		log.Info("shutting down, draining requests", "timeout", shutdownTimeout)
		if err := app.ShutdownWithTimeout(shutdownTimeout); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	}
}
