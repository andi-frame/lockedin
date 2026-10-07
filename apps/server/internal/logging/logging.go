// Package logging builds the process-wide slog logger (JSON, ARCHITECTURE §10).
package logging

import (
	"io"
	"log/slog"
	"os"
)

// New returns a JSON logger tagged with the service name. Debug level is enabled in dev.
func New(service, appEnv string) *slog.Logger {
	return newWithWriter(os.Stdout, service, appEnv)
}

func newWithWriter(w io.Writer, service, appEnv string) *slog.Logger {
	level := slog.LevelInfo
	if appEnv == "dev" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})).With("service", service)
}
