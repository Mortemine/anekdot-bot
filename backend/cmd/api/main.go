package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alexey/anekdot-bot/backend/internal/app"
	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/internal/di"
	"github.com/alexey/anekdot-bot/backend/internal/logging"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application terminated", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	injector, err := di.New(cfg, logger)
	if err != nil {
		return err
	}

	return app.Run(context.Background(), cfg, injector, logger)
}
