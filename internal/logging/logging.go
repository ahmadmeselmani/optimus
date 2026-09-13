// Package logging configures the structured logger used across Optimus.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// New builds a slog.Logger writing text-formatted logs to stderr at the
// given level ("debug", "info", "warn", "error"). Unknown levels default
// to info.
func New(level string) *slog.Logger {
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
