package main

import (
	"context"
	"log/slog"

	"github.com/alexey/anekdotus/backend/internal/adapter/postgres"
	"github.com/alexey/anekdotus/backend/internal/config"
	"github.com/alexey/anekdotus/backend/internal/entrypoint"
	"github.com/alexey/anekdotus/backend/internal/logging"
	"github.com/alexey/anekdotus/backend/internal/seed"
)

func main() {
	entrypoint.Run("seeding failed", run)
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	slog.SetDefault(logger)

	pool, err := postgres.NewPool(context.Background(), cfg, logger)
	if err != nil {
		return err
	}
	defer pool.Close()

	stats, err := seed.Run(context.Background(), pool, cfg.DataDir, logger)
	if err != nil {
		return err
	}

	logger.Info(
		"seeding completed",
		"categories", stats.Categories,
		"anekdots", stats.Anekdots,
		"skipped", stats.Skipped,
	)

	return nil
}
