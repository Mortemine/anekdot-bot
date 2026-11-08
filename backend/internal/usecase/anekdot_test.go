package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
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

	byID             domain.Anekdot
	byIDErr          error
	created          domain.Anekdot
	createErr        error
	createArg        domain.Anekdot
	authored         []domain.Anekdot
	authoredErr      error
	receivedAuthorID int64
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

func (s *anekdotRepositoryStub) ByID(context.Context, int64) (domain.Anekdot, error) {
	return s.byID, s.byIDErr
}

func (s *anekdotRepositoryStub) Create(_ context.Context, anekdot domain.Anekdot) (domain.Anekdot, error) {
	s.createArg = anekdot

	return s.created, s.createErr
}

func (s *anekdotRepositoryStub) ListByAuthor(_ context.Context, authorID int64, _ int) ([]domain.Anekdot, error) {
	s.receivedAuthorID = authorID

	return s.authored, s.authoredErr
}

type favoriteRepositoryStub struct {
	list           []domain.Anekdot
	listErr        error
	addErr         error
	removeErr      error
	added          []int64
	removed        []int64
	receivedUserID int64
}

func (s *favoriteRepositoryStub) List(_ context.Context, userID int64) ([]domain.Anekdot, error) {
	s.receivedUserID = userID

	return s.list, s.listErr
}

func (s *favoriteRepositoryStub) Add(_ context.Context, userID, anekdotID int64) error {
	s.receivedUserID = userID
	s.added = append(s.added, anekdotID)

	return s.addErr
}

func (s *favoriteRepositoryStub) Remove(_ context.Context, userID, anekdotID int64) error {
	s.receivedUserID = userID
	s.removed = append(s.removed, anekdotID)

	return s.removeErr
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
		&favoriteRepositoryStub{},
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
			service := NewAnekdotService(test.categories, test.anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

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
		service := NewAnekdotService(&categoryRepositoryStub{}, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

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
		service := NewAnekdotService(categories, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

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
		service := NewAnekdotService(categories, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

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
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		categoryID := -1
		_, err := service.Random(context.Background(), &categoryID)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Random() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("unknown category", func(t *testing.T) {
		categories := &categoryRepositoryStub{byID: map[int]domain.Category{}}
		service := NewAnekdotService(categories, &anekdotRepositoryStub{}, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		categoryID := 42
		_, err := service.Random(context.Background(), &categoryID)
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("Random() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{randomErr: errors.New("boom")}
		service := NewAnekdotService(&categoryRepositoryStub{}, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		if _, err := service.Random(context.Background(), nil); err == nil {
			t.Fatal("Random() error = nil, want error")
		}
	})
}

func TestAnekdotServiceExternal(t *testing.T) {
	service := NewAnekdotService(
		&categoryRepositoryStub{},
		&anekdotRepositoryStub{},
		&favoriteRepositoryStub{},
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

func newSubmitService(categories *categoryRepositoryStub, anekdots *anekdotRepositoryStub) *AnekdotService {
	return NewAnekdotService(categories, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())
}

func TestAnekdotServiceCreate(t *testing.T) {
	categories := &categoryRepositoryStub{byID: map[int]domain.Category{3: {ID: 3, Name: "Школьные"}}}

	t.Run("stores trimmed anecdote", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{created: domain.Anekdot{ID: 5, CategoryID: 3, Text: "свежий анекдот"}}
		service := newSubmitService(categories, anekdots)

		created, err := service.Create(context.Background(), 7, 3, "  свежий анекдот  ")
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if created.ID != 5 {
			t.Errorf("Create() id = %d, want 5", created.ID)
		}
		if anekdots.createArg.Text != "свежий анекдот" {
			t.Errorf("stored text = %q, want trimmed", anekdots.createArg.Text)
		}
		if anekdots.createArg.AuthorID == nil || *anekdots.createArg.AuthorID != 7 {
			t.Errorf("stored author = %v, want 7", anekdots.createArg.AuthorID)
		}
	})

	t.Run("rejects short text", func(t *testing.T) {
		service := newSubmitService(categories, &anekdotRepositoryStub{})

		_, err := service.Create(context.Background(), 7, 3, "коротко")
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects long text", func(t *testing.T) {
		service := newSubmitService(categories, &anekdotRepositoryStub{})

		_, err := service.Create(context.Background(), 7, 3, strings.Repeat("а", maxAnekdotLength+1))
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("rejects negative category", func(t *testing.T) {
		service := newSubmitService(categories, &anekdotRepositoryStub{})

		_, err := service.Create(context.Background(), 7, -1, "нормальный анекдот")
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("propagates conflict", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{createErr: domain.ErrConflict}
		service := newSubmitService(categories, anekdots)

		_, err := service.Create(context.Background(), 7, 3, "повторяющийся анекдот")
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("Create() error = %v, want ErrConflict", err)
		}
	})
}

func TestAnekdotServiceMine(t *testing.T) {
	anekdots := &anekdotRepositoryStub{authored: []domain.Anekdot{{ID: 1}, {ID: 2}}}
	service := newSubmitService(&categoryRepositoryStub{}, anekdots)

	mine, err := service.Mine(context.Background(), 42)
	if err != nil {
		t.Fatalf("Mine() error = %v", err)
	}
	if len(mine) != 2 {
		t.Errorf("len(mine) = %d, want 2", len(mine))
	}
	if anekdots.receivedAuthorID != 42 {
		t.Errorf("receivedAuthorID = %d, want 42", anekdots.receivedAuthorID)
	}
}

func TestAnekdotServiceFavorites(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		favorites := &favoriteRepositoryStub{list: []domain.Anekdot{{ID: 4}}}
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, favorites, &externalSourceStub{}, discardLogger())

		list, err := service.Favorites(context.Background(), 9)
		if err != nil {
			t.Fatalf("Favorites() error = %v", err)
		}
		if len(list) != 1 || favorites.receivedUserID != 9 {
			t.Errorf("Favorites() = %v, user = %d", list, favorites.receivedUserID)
		}
	})

	t.Run("add", func(t *testing.T) {
		favorites := &favoriteRepositoryStub{}
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, favorites, &externalSourceStub{}, discardLogger())

		if err := service.AddFavorite(context.Background(), 9, 4); err != nil {
			t.Fatalf("AddFavorite() error = %v", err)
		}
		if len(favorites.added) != 1 || favorites.added[0] != 4 {
			t.Errorf("added = %v, want [4]", favorites.added)
		}
	})

	t.Run("add rejects non-positive id", func(t *testing.T) {
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		if err := service.AddFavorite(context.Background(), 9, 0); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("AddFavorite() error = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("add propagates missing anecdote", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{byIDErr: domain.ErrNotFound}
		service := NewAnekdotService(&categoryRepositoryStub{}, anekdots, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		if err := service.AddFavorite(context.Background(), 9, 404); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("AddFavorite() error = %v, want ErrNotFound", err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		favorites := &favoriteRepositoryStub{}
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, favorites, &externalSourceStub{}, discardLogger())

		if err := service.RemoveFavorite(context.Background(), 9, 4); err != nil {
			t.Fatalf("RemoveFavorite() error = %v", err)
		}
		if len(favorites.removed) != 1 || favorites.removed[0] != 4 {
			t.Errorf("removed = %v, want [4]", favorites.removed)
		}
	})

	t.Run("remove rejects non-positive id", func(t *testing.T) {
		service := NewAnekdotService(&categoryRepositoryStub{}, &anekdotRepositoryStub{}, &favoriteRepositoryStub{}, &externalSourceStub{}, discardLogger())

		if err := service.RemoveFavorite(context.Background(), 9, -2); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("RemoveFavorite() error = %v, want ErrInvalidInput", err)
		}
	})
}
