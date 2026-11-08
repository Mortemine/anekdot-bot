package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type categoryRepositoryStub struct {
	categories []domain.Category
	listErr    error
	byID       map[int]domain.Category
	getErr     error
}

func (s *categoryRepositoryStub) List(context.Context) ([]domain.Category, error) {
	return s.categories, s.listErr
}

func (s *categoryRepositoryStub) Get(_ context.Context, id int) (domain.Category, error) {
	if s.getErr != nil {
		return domain.Category{}, s.getErr
	}

	category, ok := s.byID[id]
	if !ok {
		return domain.Category{}, domain.ErrNotFound
	}

	return category, nil
}

type anekdotRepositoryStub struct {
	random             domain.Anekdot
	randomErr          error
	byCategory         domain.Anekdot
	byCategoryErr      error
	counts             map[int]int
	countsErr          error
	receivedCategoryID int
	byCategoryCalled   bool
}

func (s *anekdotRepositoryStub) Random(context.Context) (domain.Anekdot, error) {
	return s.random, s.randomErr
}

func (s *anekdotRepositoryStub) RandomByCategory(_ context.Context, categoryID int) (domain.Anekdot, error) {
	s.byCategoryCalled = true
	s.receivedCategoryID = categoryID

	return s.byCategory, s.byCategoryErr
}

func (s *anekdotRepositoryStub) CountsByCategory(context.Context) (map[int]int, error) {
	return s.counts, s.countsErr
}

type externalSourceStub struct {
	text string
	err  error
}

func (s *externalSourceStub) Random(context.Context) (string, error) {
	return s.text, s.err
}

func TestAnekdotServiceListCategories(t *testing.T) {
	service := NewAnekdotService(
		&categoryRepositoryStub{
			categories: []domain.Category{{ID: 1, Name: "Разные"}, {ID: 2, Name: "Школьные"}},
		},
		&anekdotRepositoryStub{counts: map[int]int{1: 7}},
		&externalSourceStub{},
		discardLogger(),
	)

	summaries, err := service.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("ListCategories() error = %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].AnekdotCount != 7 {
		t.Errorf("category 1 count = %d, want 7", summaries[0].AnekdotCount)
	}
	if summaries[1].AnekdotCount != 0 {
		t.Errorf("category 2 count = %d, want 0", summaries[1].AnekdotCount)
	}
}

func TestAnekdotServiceListCategoriesErrors(t *testing.T) {
	tests := []struct {
		name       string
		categories *categoryRepositoryStub
		anekdots   *anekdotRepositoryStub
	}{
		{
			name:       "category list fails",
			categories: &categoryRepositoryStub{listErr: errors.New("boom")},
			anekdots:   &anekdotRepositoryStub{},
		},
		{
			name:       "counts fail",
			categories: &categoryRepositoryStub{},
			anekdots:   &anekdotRepositoryStub{countsErr: errors.New("boom")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewAnekdotService(test.categories, test.anekdots, &externalSourceStub{}, discardLogger())

			if _, err := service.ListCategories(context.Background()); err == nil {
				t.Fatal("ListCategories() error = nil, want error")
			}
		})
	}
}

func TestAnekdotServiceRandom(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		expected := domain.Anekdot{ID: 10, Text: "анекдот"}
		anekdots := &anekdotRepositoryStub{random: expected}
		service := NewAnekdotService(&categoryRepositoryStub{}, anekdots, &externalSourceStub{}, discardLogger())

		anekdot, err := service.Random(context.Background(), nil)
		if err != nil {
			t.Fatalf("Random() error = %v", err)
		}
		if anekdot != expected {
			t.Errorf("Random() = %+v, want %+v", anekdot, expected)
		}
		if anekdots.byCategoryCalled {
			t.Error("RandomByCategory was called for a global request")
		}
	})

	t.Run("by category", func(t *testing.T) {
		expected := domain.Anekdot{ID: 11, CategoryID: 3, Text: "школьный"}
		categories := &categoryRepositoryStub{byID: map[int]domain.Category{3: {ID: 3, Name: "Школьные"}}}
		anekdots := &anekdotRepositoryStub{byCategory: expected}
		service := NewAnekdotService(categories, anekdots, &externalSourceStub{}, discardLogger())

		categoryID := 3
		anekdot, err := service.Random(context.Background(), &categoryID)
		if err != nil {
			t.Fatalf("Random() error = %v", err)
		}
		if anekdot != expected {
			t.Errorf("Random() = %+v, want %+v", anekdot, expected)
		}
		if !anekdots.byCategoryCalled || anekdots.receivedCategoryID != 3 {
			t.Errorf("RandomByCategory called = %v with %d, want true with 3", anekdots.byCategoryCalled, anekdots.receivedCategoryID)
		}
	})

	t.Run("category zero is valid", func(t *testing.T) {
		expected := domain.Anekdot{ID: 12, CategoryID: 0, Text: "без категории"}
		categories := &categoryRepositoryStub{byID: map[int]domain.Category{0: {ID: 0, Name: "Вне категорий"}}}
		anekdots := &anekdotRepositoryStub{byCategory: expected}
		service := NewAnekdotService(categories, anekdots, &externalSourceStub{}, discardLogger())

		categoryID := 0
		anekdot, err := service.Random(context.Background(), &categoryID)
		if err != nil {
			t.Fatalf("Random() error = %v", err)
		}
		if anekdot != expected || anekdots.receivedCategoryID != 0 {
			t.Errorf("Random() = %+v, want %+v with category 0", anekdot, expected)
		}
	})

	t.Run("invalid category id", func(t *testing.T) {
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, &externalSourceStub{}, discardLogger())

		categoryID := -1
		_, err := service.Random(context.Background(), &categoryID)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Random() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("unknown category", func(t *testing.T) {
		categories := &categoryRepositoryStub{byID: map[int]domain.Category{}}
		service := NewAnekdotService(categories, &anekdotRepositoryStub{}, &externalSourceStub{}, discardLogger())

		categoryID := 42
		_, err := service.Random(context.Background(), &categoryID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Random() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{randomErr: errors.New("boom")}
		service := NewAnekdotService(&categoryRepositoryStub{}, anekdots, &externalSourceStub{}, discardLogger())

		if _, err := service.Random(context.Background(), nil); err == nil {
			t.Fatal("Random() error = nil, want error")
		}
	})
}

func TestAnekdotServiceExternal(t *testing.T) {
	service := NewAnekdotService(
		&categoryRepositoryStub{},
		&anekdotRepositoryStub{},
		&externalSourceStub{text: "внешний анекдот"},
		discardLogger(),
	)

	text, err := service.External(context.Background())
	if err != nil {
		t.Fatalf("External() error = %v", err)
	}
	if text != "внешний анекдот" {
		t.Errorf("External() = %q", text)
	}
}
