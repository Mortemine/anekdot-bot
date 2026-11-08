// Command migrate applies PostgreSQL migrations embedded in the binary.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	direction := flag.String("direction", "up", "migration direction: up or down")
	steps := flag.Int("steps", 0, "number of migrations to roll back (0 = all)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.PingContext(context.Background()); err != nil {
		return fmt.Errorf("pinging database: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("creating migration driver: %w", err)
	}

	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	switch *direction {
	case "up":
		err = migrator.Up()
	case "down":
		if *steps > 0 {
			err = migrator.Steps(-*steps)
		} else {
			err = migrator.Down()
		}
	default:
		return fmt.Errorf("unsupported direction %q: use up or down", *direction)
	}

	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no migrations to apply")

		return nil
	}
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	slog.Info("migrations applied", "direction", *direction)

	return nil
}
