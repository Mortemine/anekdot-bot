package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/alexey/anekdotus/backend/internal/config"
	"github.com/alexey/anekdotus/backend/internal/entrypoint"
	"github.com/alexey/anekdotus/backend/migrations"
)

const (
	directionUp      = "up"
	directionDown    = "down"
	directionVersion = "version"
)

type options struct {
	direction string
	steps     int
}

func main() {
	entrypoint.Run("migration failed", run)
}

func parseOptions() options {
	direction := flag.String("direction", directionUp, "migration direction: up, down or version")
	steps := flag.Int("steps", 0, "number of migrations to roll back (0 = all)")
	flag.Parse()

	return options{direction: *direction, steps: *steps}
}

func run() error {
	opts := parseOptions()

	if err := validateDirection(opts.direction); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	db, err := openDatabase(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	migrator, err := buildMigrator(db, migrations.FS)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	return finish(apply(migrator, opts.direction, opts.steps), opts.direction)
}

func validateDirection(direction string) error {
	switch direction {
	case directionUp, directionDown, directionVersion:
		return nil
	default:
		return fmt.Errorf("unsupported direction %q: use up, down or version", direction)
	}
}

func openDatabase(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

func buildMigrator(db *sql.DB, fsys fs.FS) (*migrate.Migrate, error) {
	migrationSource, err := iofs.New(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("creating migration source: %w", err)
	}

	driver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{})
	if err != nil {
		_ = migrationSource.Close()

		return nil, fmt.Errorf("creating migration driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", migrationSource, "pgx5", driver)
	if err != nil {
		return nil, fmt.Errorf("creating migrator: %w", err)
	}

	return migrator, nil
}

func finish(err error, direction string) error {
	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no migrations to apply")

		return nil
	}

	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	slog.Info("migrations applied", "direction", direction)

	return nil
}

func apply(migrator *migrate.Migrate, direction string, steps int) error {
	switch direction {
	case directionUp:
		return migrator.Up()
	case directionDown:
		if steps > 0 {
			return migrator.Steps(-steps)
		}

		return migrator.Down()
	case directionVersion:
		return printVersion(migrator)
	default:
		return fmt.Errorf("unsupported direction %q: use up, down or version", direction)
	}
}

func printVersion(migrator *migrate.Migrate) error {
	version, dirty, err := migrator.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		slog.Info("no migrations applied")

		return nil
	}
	if err != nil {
		return fmt.Errorf("reading migration version: %w", err)
	}

	slog.Info("migration version", "version", version, "dirty", dirty)

	return nil
}
