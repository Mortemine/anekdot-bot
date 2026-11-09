package seed

import (
	"strings"
	"testing"
)

func TestParseCategories(t *testing.T) {
	input := "0, Вне категорий\n1, Разные\n\n  2 ,  Школьные  \n"

	categories, err := ParseCategories(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseCategories() error = %v", err)
	}

	if len(categories) != 3 {
		t.Fatalf("len(categories) = %d, want 3", len(categories))
	}
	if categories[0].ID != 0 || categories[0].Name != "Вне категорий" {
		t.Errorf("category 0 = %+v", categories[0])
	}
	if categories[2].ID != 2 || categories[2].Name != "Школьные" {
		t.Errorf("category 2 = %+v", categories[2])
	}
}

func TestParseCategoriesErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "missing comma", input: "1 Разные", want: "categories: line 1: missing comma separator"},
		{name: "invalid id", input: "abc, Разные", want: "categories: line 1: invalid category id"},
		{name: "empty name", input: "1,   ", want: "categories: line 1: category 1 has an empty name"},
		{name: "duplicate id", input: "1, Разные\n1, Другие", want: "categories: line 2: duplicate id 1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseCategories(strings.NewReader(test.input))
			if err == nil {
				t.Fatal("ParseCategories() error = nil, want error")
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want it to contain %q", err, test.want)
			}
		})
	}
}

func TestParseAnekdots(t *testing.T) {
	input := strings.Join([]string{
		` 1, Первый\nвторой`,
		``,
		` 1, &quot;Привет&quot;`,
		` 2, Чужая категория`,
		` 3, Тоже неизвестная`,
		` abc, плохой id`,
		` 1, Без запятой в тексте`,
		` 1,    `,
		`, без категории`,
	}, "\n")

	known := map[int]struct{}{1: {}}

	records, stats, err := ParseAnekdots(strings.NewReader(input), known)
	if err != nil {
		t.Fatalf("ParseAnekdots() error = %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("len(records) = %d, want 3: %+v", len(records), records)
	}
	if records[0].CategoryID != 1 || records[0].Text != "Первый\nвторой" {
		t.Errorf("record 0 = %+v", records[0])
	}
	if records[1].Text != `"Привет"` {
		t.Errorf("record 1 text = %q", records[1].Text)
	}
	if records[2].Text != "Без запятой в тексте" {
		t.Errorf("record 2 text = %q", records[2].Text)
	}

	if stats.Total != 9 {
		t.Errorf("Total = %d, want 9", stats.Total)
	}
	if stats.Imported != 3 {
		t.Errorf("Imported = %d, want 3", stats.Imported)
	}
	if stats.Skipped != 6 {
		t.Errorf("Skipped = %d, want 6", stats.Skipped)
	}
}

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "escaped line breaks", in: `Один\nДва`, want: "Один\nДва"},
		{name: "entities", in: `&quot;Привет&quot; &amp; пока`, want: `"Привет" & пока`},
		{name: "trim", in: "   текст   ", want: "текст"},
		{name: "empty", in: "   ", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := normalizeText(test.in); got != test.want {
				t.Errorf("normalizeText(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}
