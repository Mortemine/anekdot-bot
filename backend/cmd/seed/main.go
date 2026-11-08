package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/alexey/anekdot-bot/backend/internal/adapter/postgres"
	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/internal/logging"
	"github.com/alexey/anekdot-bot/backend/internal/seed"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seeding failed", "error", err)
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
