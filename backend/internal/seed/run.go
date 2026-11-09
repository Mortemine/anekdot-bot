package seed

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const defaultBatchSize = 500

type Stats struct {
	Categories int
	Anekdots   int
	Skipped    int
}

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
	processed := 0
	remaining := records

	for len(remaining) != 0 {
		size := min(defaultBatchSize, len(remaining))

		count, err := insertBatch(ctx, pool, query, remaining[:size])
		if err != nil {
			return 0, err
		}

		inserted += count
		processed += size
		remaining = remaining[size:]

		if logger != nil && shouldLogProgress(processed, len(records)) {
			logger.Info("seeding anekdots", "processed", processed, "total", len(records))
		}
	}

	return inserted, nil
}

func shouldLogProgress(processed, total int) bool {
	return processed == total || processed%20_000 == 0
}

func insertBatch(ctx context.Context, pool *pgxpool.Pool, query string, records []Anekdot) (int, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("starting seed transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	return storeBatch(ctx, transactionSender{tx: tx}, query, records)
}

type batchTransaction interface {
	SendBatch(ctx context.Context, batch *pgx.Batch) batchResults
	Commit(ctx context.Context) error
}

type transactionSender struct {
	tx pgx.Tx
}

func (t transactionSender) SendBatch(ctx context.Context, batch *pgx.Batch) batchResults {
	return t.tx.SendBatch(ctx, batch)
}

func (t transactionSender) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func storeBatch(ctx context.Context, tx batchTransaction, query string, records []Anekdot) (int, error) {
	batch := &pgx.Batch{}
	for _, record := range records {
		batch.Queue(query, record.CategoryID, record.Text)
	}

	inserted, err := collectBatch(tx.SendBatch(ctx, batch), len(records))
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("committing seed transaction: %w", err)
	}

	return inserted, nil
}

type batchResults interface {
	Exec() (pgconn.CommandTag, error)
	Close() error
}

func collectBatch(results batchResults, total int) (int, error) {
	inserted := 0

	var execErr error

	for range total {
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

	return inserted, nil
}
