package seed

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestReadCategories(t *testing.T) {
	t.Run("reads categories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "categories.txt", "0, Вне категорий\n1, Разные\n")

		categories, err := readCategories(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(categories) != 2 || categories[1].Name != "Разные" {
			t.Fatalf("unexpected categories: %+v", categories)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, err := readCategories(t.TempDir()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("malformed file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "categories.txt", "broken\n")

		if _, err := readCategories(dir); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestReadAnekdots(t *testing.T) {
	t.Run("reads anecdotes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "anek_data.txt", "1, Приходит к Ельцину Мизулина и говорит\n1, Второй анекдот про Ельцина\n")

		known := map[int]struct{}{1: {}}
		records, stats, err := readAnekdots(dir, known)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(records) != 2 {
			t.Fatalf("unexpected records: %+v", records)
		}

		if stats.Imported != 2 {
			t.Fatalf("unexpected stats: %+v", stats)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if _, _, err := readAnekdots(t.TempDir(), map[int]struct{}{1: {}}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("malformed lines are skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "anek_data.txt", "broken-line\nnot-a-number, Анекдот\n\n1, \n")

		records, stats, err := readAnekdots(dir, map[int]struct{}{1: {}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(records) != 0 || stats.Skipped != 4 {
			t.Fatalf("unexpected result: %+v %+v", records, stats)
		}
	})

	t.Run("unknown category is skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "anek_data.txt", "99, Анекдот про Ельцина и Мизулину\n")

		records, stats, err := readAnekdots(dir, map[int]struct{}{1: {}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(records) != 0 || stats.Skipped != 1 {
			t.Fatalf("unexpected result: %+v %+v", records, stats)
		}
	})
}

func TestShouldLogProgress(t *testing.T) {
	tests := []struct {
		name      string
		processed int
		total     int
		want      bool
	}{
		{name: "mid batch", processed: 500, total: 1200},
		{name: "final batch", processed: 1200, total: 1200, want: true},
		{name: "round ten thousand", processed: 20_000, total: 50_000, want: true},
		{name: "round fifty thousand", processed: 50_000, total: 50_000, want: true},
		{name: "empty input", processed: 0, total: 0, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldLogProgress(test.processed, test.total); got != test.want {
				t.Fatalf("shouldLogProgress(%d, %d) = %v, want %v", test.processed, test.total, got, test.want)
			}
		})
	}
}

func TestRunReportsMissingData(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	_, err := Run(t.Context(), nil, t.TempDir(), logger)
	if err == nil || !strings.Contains(err.Error(), "categories") {
		t.Fatalf("unexpected error: %v", err)
	}
}
