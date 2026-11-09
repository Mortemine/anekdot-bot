package entrypoint

import (
	"log/slog"
	"os"
)

func Run(failureMessage string, run func() error) {
	if err := run(); err != nil {
		slog.Error(failureMessage, "error", err)
		os.Exit(1)
	}
}
