package main

import (
	"context"
	"log/slog"

	"github.com/alexey/anekdotus/backend/internal/app"
	"github.com/alexey/anekdotus/backend/internal/config"
	"github.com/alexey/anekdotus/backend/internal/di"
	"github.com/alexey/anekdotus/backend/internal/entrypoint"
	"github.com/alexey/anekdotus/backend/internal/logging"
)

func main() {
	entrypoint.Run("application terminated", run)
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
