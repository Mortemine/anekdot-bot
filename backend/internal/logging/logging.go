// Package logging builds the application logger.
package logging

import (
	"log/slog"
	"os"
)

// New returns a structured logger writing to stdout.
func New(level slog.Level, format string) *slog.Logger {
	options := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, options)
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, options)
	}

	return slog.New(handler)
}
