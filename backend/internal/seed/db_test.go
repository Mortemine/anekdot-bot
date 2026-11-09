package seed

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const (
	seedTestDSN       = "postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable"
	seedCategoryBase  = 9100
	unknownCategoryID = 1 << 20
)

func seedDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}

	return seedTestDSN
}

func seedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), seedDSN())
	if err != nil {
		t.Skipf("skipping database test: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()

		t.Skipf("skipping database test: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func seedCategory(t *testing.T, pool *pgxpool.Pool, id int, name string) {
	t.Helper()

	ctx := context.Background()
	_, err := pool.Exec(ctx, "DELETE FROM anekdots WHERE category_id = $1", id)
	if err != nil {
		t.Fatalf("cleaning anekdots: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO categories (id, name) VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		id,
		name,
	)
	if err != nil {
		t.Fatalf("creating category: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()

		_, _ = pool.Exec(cleanupCtx, "DELETE FROM anekdots WHERE category_id = $1", id)
		_, _ = pool.Exec(cleanupCtx, "DELETE FROM categories WHERE id = $1", id)
	})
}

func TestUpsertCategories(t *testing.T) {
	pool := seedPool(t)
	ctx := context.Background()

	categories := []domain.Category{
		{ID: seedCategoryBase, Name: "Первая"},
		{ID: seedCategoryBase + 1, Name: "Вторая"},
	}

	t.Cleanup(func() {
		for _, category := range categories {
			_, _ = pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", category.ID)
		}
	})

	if err := upsertCategories(ctx, pool, categories); err != nil {
		t.Fatalf("upserting categories: %v", err)
	}

	renamed := []domain.Category{{ID: seedCategoryBase, Name: "Переименованная"}}
	if err := upsertCategories(ctx, pool, renamed); err != nil {
		t.Fatalf("upserting renamed category: %v", err)
	}

	var name string
	if err := pool.QueryRow(ctx, "SELECT name FROM categories WHERE id = $1", seedCategoryBase).Scan(&name); err != nil {
		t.Fatalf("reading category: %v", err)
	}

	if name != "Переименованная" {
		t.Fatalf("unexpected category name: %q", name)
	}

	invalid := []domain.Category{{ID: seedCategoryBase + 2, Name: ""}}
	if err := upsertCategories(ctx, pool, invalid); err == nil {
		t.Fatal("expected error for an empty category name")
	}
}

func TestInsertAnekdots(t *testing.T) {
	pool := seedPool(t)
	ctx := context.Background()
	categoryID := seedCategoryBase + 3

	seedCategory(t, pool, categoryID, "Вставка")
	logger := slog.New(slog.DiscardHandler)
	records := []Anekdot{
		{CategoryID: categoryID, Text: "Первый анекдот для вставки"},
		{CategoryID: categoryID, Text: "Второй анекдот для вставки"},
	}

	inserted, err := insertAnekdots(ctx, pool, records, logger)
	if err != nil {
		t.Fatalf("inserting anekdots: %v", err)
	}

	if inserted != len(records) {
		t.Fatalf("expected %d inserted, got %d", len(records), inserted)
	}

	repeated, err := insertAnekdots(ctx, pool, records, nil)
	if err != nil {
		t.Fatalf("re-inserting anekdots: %v", err)
	}

	if repeated != 0 {
		t.Fatalf("expected no duplicates inserted, got %d", repeated)
	}

	empty, err := insertAnekdots(ctx, pool, nil, logger)
	if err != nil {
		t.Fatalf("inserting an empty batch: %v", err)
	}

	if empty != 0 {
		t.Fatalf("expected nothing inserted for an empty batch, got %d", empty)
	}

	broken := []Anekdot{{CategoryID: unknownCategoryID, Text: "Анекдот в несуществующую категорию"}}
	if _, err := insertAnekdots(ctx, pool, broken, logger); err == nil {
		t.Fatal("expected a foreign key error")
	}
}

func TestInsertBatchRejectsClosedPool(t *testing.T) {
	seedPool(t)

	ctx := context.Background()

	closed, err := pgxpool.New(ctx, seedDSN())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	closed.Close()

	count, err := insertBatch(ctx, closed, "SELECT 1", nil)
	if err == nil {
		t.Fatal("expected an error for a closed pool")
	}

	if count != 0 {
		t.Fatalf("insertBatch() = %d, want 0", count)
	}
}

func TestInsertAnekdotsReportsBatchFailure(t *testing.T) {
	seedPool(t)

	ctx := context.Background()

	closed, err := pgxpool.New(ctx, seedDSN())
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	closed.Close()

	records := []Anekdot{{CategoryID: seedCategoryBase, Text: "закрытый пул"}}

	inserted, err := insertAnekdots(ctx, closed, records, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected an error for a closed pool")
	}

	if inserted != 0 {
		t.Fatalf("insertAnekdots() = %d, want 0", inserted)
	}
}

func TestInsertAnekdotsProcessesEveryBatch(t *testing.T) {
	pool := seedPool(t)
	seedCategory(t, pool, seedCategoryBase, "Пакеты")

	ctx := context.Background()
	records := make([]Anekdot, defaultBatchSize+1)

	for index := range records {
		records[index] = Anekdot{CategoryID: seedCategoryBase, Text: fmt.Sprintf("пакет %d", index)}
	}

	inserted, err := insertAnekdots(ctx, pool, records, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("insertAnekdots() error = %v", err)
	}

	if inserted != len(records) {
		t.Fatalf("insertAnekdots() = %d, want %d", inserted, len(records))
	}
}

func TestInsertAnekdotsRejectsUnknownCategory(t *testing.T) {
	pool := seedPool(t)

	records := []Anekdot{{CategoryID: unknownCategoryID, Text: "неизвестная категория"}}

	inserted, err := insertAnekdots(context.Background(), pool, records, slog.New(slog.DiscardHandler))
	if err == nil {
		t.Fatal("expected a foreign key error")
	}

	if inserted != 0 {
		t.Fatalf("insertAnekdots() = %d, want 0", inserted)
	}
}

func TestRun(t *testing.T) {
	pool := seedPool(t)
	ctx := context.Background()
	dir := t.TempDir()
	categoryID := seedCategoryBase + 5

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM anekdots WHERE category_id = $1", categoryID)
		_, _ = pool.Exec(ctx, "DELETE FROM categories WHERE id = $1", categoryID)
	})

	writeFile(t, dir, "categories.txt", fmt.Sprintf("%d, Загруженная\n", categoryID))
	writeFile(
		t,
		dir,
		"anek_data.txt",
		fmt.Sprintf("%d, Анекдот для полного прогона\n%d, Второй анекдот для прогона\nbad, строка\n", categoryID, categoryID),
	)

	stats, err := Run(ctx, pool, dir, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("running seed: %v", err)
	}

	if stats.Categories != 1 || stats.Anekdots != 2 || stats.Skipped != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}

	t.Run("missing categories file", func(t *testing.T) {
		if _, err := Run(ctx, pool, t.TempDir(), nil); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("missing anekdots file", func(t *testing.T) {
		broken := t.TempDir()
		writeFile(t, broken, "categories.txt", fmt.Sprintf("%d, Загруженная\n", categoryID))

		if _, err := Run(ctx, pool, broken, nil); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("category rejected by the database", func(t *testing.T) {
		broken := t.TempDir()
		writeFile(t, broken, "categories.txt", fmt.Sprintf("%d, %s\n", categoryID+9, strings.Repeat("я", 201)))
		writeFile(t, broken, "anek_data.txt", fmt.Sprintf("%d, Анекдот\n", categoryID+9))

		if _, err := Run(ctx, pool, broken, nil); err == nil {
			t.Fatal("expected an error")
		}
	})
}
