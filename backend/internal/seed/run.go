package seed

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const defaultBatchSize = 500

// Stats summarizes a completed import.
type Stats struct {
	Categories int
	Anekdots   int
	Skipped    int
}

// Run imports categories and anecdotes from dataDir. The import is idempotent:
// categories are upserted and duplicates are ignored by a unique index.
func Run(ctx context.Context, pool *pgxpool.Pool, dataDir string, logger *slog.Logger) (Stats, error) {
	categories, err := readCategories(dataDir)
	if err != nil {
		return Stats{}, err
	}

	if err := upsertCategories(ctx, pool, categories); err != nil {
		return Stats{}, err
	}

	known := make(map[int]struct{}, len(categories))
	for _, category := range categories {
		known[category.ID] = struct{}{}
	}

	records, parseStats, err := readAnekdots(dataDir, known)
	if err != nil {
		return Stats{}, err
	}

	inserted, err := insertAnekdots(ctx, pool, records, logger)
	if err != nil {
		return Stats{}, err
	}

	return Stats{
		Categories: len(categories),
		Anekdots:   inserted,
		Skipped:    parseStats.Skipped,
	}, nil
}

func readCategories(dataDir string) ([]domain.Category, error) {
	file, err := os.DirFS(dataDir).Open("categories.txt")
	if err != nil {
		return nil, fmt.Errorf("opening categories file: %w", err)
	}
	defer func() { _ = file.Close() }()

	categories, err := ParseCategories(file)
	if err != nil {
		return nil, err
	}

	return categories, nil
}

func readAnekdots(dataDir string, known map[int]struct{}) ([]Anekdot, ParseStats, error) {
	file, err := os.DirFS(dataDir).Open("anek_data.txt")
	if err != nil {
		return nil, ParseStats{}, fmt.Errorf("opening anekdots file: %w", err)
	}
	defer func() { _ = file.Close() }()

	return ParseAnekdots(file, known)
}

func upsertCategories(ctx context.Context, pool *pgxpool.Pool, categories []domain.Category) error {
	const query = `
		INSERT INTO categories (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`

	batch := &pgx.Batch{}
	for _, category := range categories {
		batch.Queue(query, category.ID, category.Name)
	}

	results := pool.SendBatch(ctx, batch)
	defer func() { _ = results.Close() }()

	for index := range categories {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("upserting category %d: %w", categories[index].ID, err)
		}
	}

	return nil
}

func insertAnekdots(
	ctx context.Context,
	pool *pgxpool.Pool,
	records []Anekdot,
	logger *slog.Logger,
) (int, error) {
	const query = `
		INSERT INTO anekdots (category_id, text)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`

	inserted := 0

	for start := 0; start < len(records); start += defaultBatchSize {
		end := min(start+defaultBatchSize, len(records))

		count, err := insertBatch(ctx, pool, query, records[start:end])
		if err != nil {
			return 0, err
		}
		inserted += count

		if logger != nil && (end == len(records) || end%20_000 == 0) {
			logger.Info("seeding anekdots", "processed", end, "total", len(records))
		}
	}

	return inserted, nil
}

func insertBatch(ctx context.Context, pool *pgxpool.Pool, query string, records []Anekdot) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("starting seed transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	batch := &pgx.Batch{}
	for _, record := range records {
		batch.Queue(query, record.CategoryID, record.Text)
	}

	results := tx.SendBatch(ctx, batch)

	inserted := 0

	var execErr error
	for range records {
		tag, err := results.Exec()
		if err != nil {
			execErr = err

			break
		}
		inserted += int(tag.RowsAffected())
	}

	closeErr := results.Close()
	if execErr != nil {
		return 0, fmt.Errorf("inserting anekdot batch: %w", execErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("closing anekdot batch: %w", closeErr)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing seed transaction: %w", err)
	}

	return inserted, nil
}
