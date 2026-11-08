package usecase

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type categoryRepositoryStub struct {
	items   []domain.Category
	listErr error
	getErr  error
}

func (s *categoryRepositoryStub) List(ctx context.Context) ([]domain.Category, error) {
	return s.items, s.listErr
}

func (s *categoryRepositoryStub) Get(ctx context.Context, id int) (domain.Category, error) {
	if s.getErr != nil {
		return domain.Category{}, s.getErr
	}

	return domain.Category{ID: id, Name: "Категория"}, nil
}

type anekdotRepositoryStub struct {
	listed      []domain.Anekdot
	listErr     error
	byID        *domain.Anekdot
	byIDErr     error
	created     domain.Anekdot
	createErr   error
	createdWith domain.Anekdot
	counts      map[int]int
	countsErr   error
	receivedCat *int
	receivedLim int
	receivedOff int
}

func (s *anekdotRepositoryStub) Random(ctx context.Context) (domain.Anekdot, error) {
	if s.byIDErr != nil {
		return domain.Anekdot{}, s.byIDErr
	}

	return s.created, nil
}

func (s *anekdotRepositoryStub) RandomByCategory(ctx context.Context, categoryID int) (domain.Anekdot, error) {
	s.receivedCat = &categoryID

	return s.Random(ctx)
}

func (s *anekdotRepositoryStub) CountsByCategory(ctx context.Context) (map[int]int, error) {
	return s.counts, s.countsErr
}

func (s *anekdotRepositoryStub) ByID(ctx context.Context, id int64) (domain.Anekdot, error) {
	if s.byIDErr != nil {
		return domain.Anekdot{}, s.byIDErr
	}

	if s.byID == nil {
		return domain.Anekdot{}, domain.ErrNotFound
	}

	return *s.byID, nil
}

func (s *anekdotRepositoryStub) Create(ctx context.Context, anekdot domain.Anekdot) (domain.Anekdot, error) {
	s.createdWith = anekdot

	if s.createErr != nil {
		return domain.Anekdot{}, s.createErr
	}

	return s.created, nil
}

func (s *anekdotRepositoryStub) ListByAuthor(
	ctx context.Context,
	authorID int64,
	limit int,
) ([]domain.Anekdot, error) {
	return s.listed, s.listErr
}

func (s *anekdotRepositoryStub) List(
	ctx context.Context,
	categoryID *int,
	limit int,
	offset int,
) ([]domain.Anekdot, error) {
	s.receivedCat = categoryID
	s.receivedLim = limit
	s.receivedOff = offset

	return s.listed, s.listErr
}

type reactionRepositoryStub struct {
	list           []domain.Anekdot
	listErr        error
	addErr         error
	removeErr      error
	added          []int64
	removed        []int64
	receivedUserID int64
}

func (s *reactionRepositoryStub) List(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	s.receivedUserID = userID

	return s.list, s.listErr
}

func (s *reactionRepositoryStub) Add(ctx context.Context, userID, anekdotID int64) error {
	s.receivedUserID = userID
	s.added = append(s.added, anekdotID)

	return s.addErr
}

func (s *reactionRepositoryStub) Remove(ctx context.Context, userID, anekdotID int64) error {
	s.receivedUserID = userID
	s.removed = append(s.removed, anekdotID)

	return s.removeErr
}

type externalSourceStub struct {
	text   string
	err    error
	called int
}

func (s *externalSourceStub) Random(ctx context.Context) (string, error) {
	s.called++

	return s.text, s.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func intPtr(value int) *int {
	return &value
}

type serviceDeps struct {
	categories domain.CategoryRepository
	anekdots   domain.AnekdotRepository
	likes      domain.LikeRepository
	favorites  domain.FavoriteRepository
	external   domain.ExternalAnekdotSource
}

func newTestService(deps serviceDeps) *AnekdotService {
	if deps.categories == nil {
		deps.categories = &categoryRepositoryStub{}
	}

	if deps.anekdots == nil {
		deps.anekdots = &anekdotRepositoryStub{}
	}

	if deps.likes == nil {
		deps.likes = &reactionRepositoryStub{}
	}

	if deps.favorites == nil {
		deps.favorites = &reactionRepositoryStub{}
	}

	if deps.external == nil {
		deps.external = &externalSourceStub{}
	}

	return NewAnekdotService(
		deps.categories,
		deps.anekdots,
		deps.likes,
		deps.favorites,
		deps.external,
		discardLogger(),
	)
}

func TestAnekdotServiceListCategories(t *testing.T) {
	t.Run("lists categories", func(t *testing.T) {
		categories := &categoryRepositoryStub{items: []domain.Category{{ID: 1, Name: "Работа"}}}
		anekdots := &anekdotRepositoryStub{counts: map[int]int{1: 3}}
		service := newTestService(serviceDeps{categories: categories, anekdots: anekdots})

		got, err := service.ListCategories(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || got[0].Name != "Работа" || got[0].AnekdotCount != 3 {
			t.Fatalf("unexpected categories: %+v", got)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.ListCategories(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceAnekdots(t *testing.T) {
	t.Run("lists category page", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{listed: []domain.Anekdot{{ID: 1, Text: "текст"}}}
		service := newTestService(serviceDeps{anekdots: anekdots})

		got, err := service.Anekdots(context.Background(), intPtr(2), 10, 20)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || got[0].ID != 1 {
			t.Fatalf("unexpected anekdots: %+v", got)
		}

		if anekdots.receivedCat == nil || *anekdots.receivedCat != 2 {
			t.Fatalf("unexpected category filter: %v", anekdots.receivedCat)
		}

		if anekdots.receivedLim != 10 || anekdots.receivedOff != 20 {
			t.Fatalf("unexpected page: %d %d", anekdots.receivedLim, anekdots.receivedOff)
		}
	})

	t.Run("lists everything without a category", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: anekdots})

		if _, err := service.Anekdots(context.Background(), nil, 0, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if anekdots.receivedCat != nil {
			t.Fatalf("unexpected category filter: %v", anekdots.receivedCat)
		}

		if anekdots.receivedLim != categoryPageSize || anekdots.receivedOff != 0 {
			t.Fatalf("unexpected page: %d %d", anekdots.receivedLim, anekdots.receivedOff)
		}
	})

	t.Run("caps the page size", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: anekdots})

		if _, err := service.Anekdots(context.Background(), intPtr(1), 1000, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if anekdots.receivedLim != maxCategoryPageSize {
			t.Fatalf("unexpected limit: %d", anekdots.receivedLim)
		}
	})
}

func TestAnekdotServiceAnekdotsErrors(t *testing.T) {
	t.Run("rejects negative offset", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		_, err := service.Anekdots(context.Background(), nil, 10, -1)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})

	t.Run("defaults negative limit", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: anekdots})

		if _, err := service.Anekdots(context.Background(), nil, -5, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if anekdots.receivedLim != categoryPageSize {
			t.Fatalf("unexpected limit: %d", anekdots.receivedLim)
		}
	})

	t.Run("rejects unknown category", func(t *testing.T) {
		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{getErr: domain.ErrNotFound}})

		if _, err := service.Anekdots(context.Background(), intPtr(7), 10, 0); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Anekdots(context.Background(), nil, 10, 0); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceRandom(t *testing.T) {
	t.Run("returns random anecdote", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{created: domain.Anekdot{ID: 5, Text: "случайный"}}
		service := newTestService(serviceDeps{anekdots: anekdots})

		got, err := service.Random(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID != 5 || anekdots.receivedCat != nil {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("uses category", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: anekdots})

		categoryID := 3
		if _, err := service.Random(context.Background(), &categoryID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if anekdots.receivedCat == nil || *anekdots.receivedCat != 3 {
			t.Fatalf("unexpected category: %v", anekdots.receivedCat)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byIDErr: errors.New("boom")}})

		if _, err := service.Random(context.Background(), nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("propagates category repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byIDErr: errors.New("boom")}})

		categoryID := 3
		if _, err := service.Random(context.Background(), &categoryID); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceExternal(t *testing.T) {
	t.Run("returns external anecdote", func(t *testing.T) {
		external := &externalSourceStub{text: "внешний"}
		service := newTestService(serviceDeps{external: external})

		got, err := service.External(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != "внешний" || external.called != 1 {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		service := newTestService(serviceDeps{external: &externalSourceStub{err: errors.New("boom")}})

		if _, err := service.External(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceCreate(t *testing.T) {
	t.Run("creates anecdote", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{created: domain.Anekdot{ID: 2, Text: "нормальный анекдот"}}
		service := newTestService(serviceDeps{anekdots: anekdots})

		got, err := service.Create(context.Background(), 11, 1, "  нормальный анекдот  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.ID != 2 {
			t.Fatalf("unexpected result: %+v", got)
		}

		if anekdots.createdWith.Text != "нормальный анекдот" || anekdots.createdWith.CategoryID != 1 {
			t.Fatalf("unexpected payload: %+v", anekdots.createdWith)
		}

		if anekdots.createdWith.AuthorID == nil || *anekdots.createdWith.AuthorID != 11 {
			t.Fatalf("unexpected author: %v", anekdots.createdWith.AuthorID)
		}
	})

	t.Run("rejects short text", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		if _, err := service.Create(context.Background(), 1, 1, "коротко"); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})

	t.Run("rejects long text", func(t *testing.T) {
		service := newTestService(serviceDeps{})
		text := strings.Repeat("а", maxAnekdotLength+1)

		if _, err := service.Create(context.Background(), 1, 1, text); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})

	t.Run("rejects unknown category", func(t *testing.T) {
		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{getErr: domain.ErrNotFound}})

		_, err := service.Create(context.Background(), 1, 5, "достаточно длинный анекдот")
		if !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})
}

func TestAnekdotServiceCreateConflicts(t *testing.T) {
	t.Run("propagates duplicate error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{createErr: domain.ErrConflict}})

		_, err := service.Create(context.Background(), 1, 1, "достаточно длинный анекдот")
		if !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("expected conflict, got %v", err)
		}
	})

	t.Run("propagates storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{createErr: errors.New("boom")}})

		_, err := service.Create(context.Background(), 1, 1, "достаточно длинный анекдот")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceMine(t *testing.T) {
	t.Run("returns own anecdotes", func(t *testing.T) {
		anekdots := &anekdotRepositoryStub{listed: []domain.Anekdot{{ID: 4}}}
		service := newTestService(serviceDeps{anekdots: anekdots})

		got, err := service.Mine(context.Background(), 9)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || got[0].ID != 4 {
			t.Fatalf("unexpected anekdots: %+v", got)
		}
	})
}

func TestAnekdotServiceLikes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		likes := &reactionRepositoryStub{list: []domain.Anekdot{{ID: 4, CategoryID: 1, Text: "лайкнутый"}}}
		service := newTestService(serviceDeps{likes: likes})

		got, err := service.Likes(context.Background(), 9)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || got[0].ID != 4 || likes.receivedUserID != 9 {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("list failure", func(t *testing.T) {
		service := newTestService(serviceDeps{likes: &reactionRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Likes(context.Background(), 9); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceLikeMutations(t *testing.T) {
	byID := domain.Anekdot{ID: 4, CategoryID: 1, Text: "лайкнутый"}

	t.Run("add", func(t *testing.T) {
		likes := &reactionRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byID: &byID}, likes: likes})

		if err := service.AddLike(context.Background(), 9, 4); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(likes.added) != 1 || likes.added[0] != 4 || likes.receivedUserID != 9 {
			t.Fatalf("unexpected calls: %+v", likes.added)
		}
	})

	t.Run("add rejects non-positive id", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		if err := service.AddLike(context.Background(), 9, 0); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})

	t.Run("add propagates missing anecdote", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byIDErr: domain.ErrNotFound}})

		if err := service.AddLike(context.Background(), 9, 4); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		likes := &reactionRepositoryStub{}
		service := newTestService(serviceDeps{likes: likes})

		if err := service.RemoveLike(context.Background(), 9, 4); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(likes.removed) != 1 || likes.removed[0] != 4 || likes.receivedUserID != 9 {
			t.Fatalf("unexpected calls: %+v", likes.removed)
		}
	})

	t.Run("remove rejects non-positive id", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		if err := service.RemoveLike(context.Background(), 9, -1); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})
}

func TestAnekdotServiceFavorites(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		favorites := &reactionRepositoryStub{list: []domain.Anekdot{{ID: 5, CategoryID: 2, Text: "избранный"}}}
		service := newTestService(serviceDeps{favorites: favorites})

		got, err := service.Favorites(context.Background(), 9)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(got) != 1 || got[0].ID != 5 || favorites.receivedUserID != 9 {
			t.Fatalf("unexpected result: %+v", got)
		}
	})

	t.Run("list failure", func(t *testing.T) {
		service := newTestService(serviceDeps{favorites: &reactionRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Favorites(context.Background(), 9); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAnekdotServiceFavoriteMutations(t *testing.T) {
	byID := domain.Anekdot{ID: 5, CategoryID: 2, Text: "избранный"}

	t.Run("add", func(t *testing.T) {
		favorites := &reactionRepositoryStub{}
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byID: &byID}, favorites: favorites})

		if err := service.AddFavorite(context.Background(), 9, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(favorites.added) != 1 || favorites.added[0] != 5 || favorites.receivedUserID != 9 {
			t.Fatalf("unexpected calls: %+v", favorites.added)
		}
	})

	t.Run("add rejects non-positive id", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		if err := service.AddFavorite(context.Background(), 9, 0); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})

	t.Run("add propagates missing anecdote", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{byIDErr: domain.ErrNotFound}})

		if err := service.AddFavorite(context.Background(), 9, 5); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("remove", func(t *testing.T) {
		favorites := &reactionRepositoryStub{}
		service := newTestService(serviceDeps{favorites: favorites})

		if err := service.RemoveFavorite(context.Background(), 9, 5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(favorites.removed) != 1 || favorites.removed[0] != 5 || favorites.receivedUserID != 9 {
			t.Fatalf("unexpected calls: %+v", favorites.removed)
		}
	})

	t.Run("remove rejects non-positive id", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		if err := service.RemoveFavorite(context.Background(), 9, 0); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected invalid input, got %v", err)
		}
	})
}

func TestAnekdotServiceFavoritesAreSeparate(t *testing.T) {
	likes := &reactionRepositoryStub{list: []domain.Anekdot{{ID: 6}}}
	favorites := &reactionRepositoryStub{list: []domain.Anekdot{{ID: 5}}}
	service := newTestService(serviceDeps{likes: likes, favorites: favorites})

	if _, err := service.Likes(context.Background(), 9); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := service.Favorites(context.Background(), 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 || got[0].ID != 5 {
		t.Fatalf("unexpected favorites: %+v", got)
	}
}

func TestAnekdotServiceErrorPropagation(t *testing.T) {
	t.Run("categories repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.ListCategories(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("category counts repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{countsErr: errors.New("boom")}})

		if _, err := service.ListCategories(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("mine repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Mine(context.Background(), 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("create rejects negative category", func(t *testing.T) {
		service := newTestService(serviceDeps{})

		categoryID := -1
		if _, err := service.Create(context.Background(), 1, categoryID, "Очень смешной анекдот"); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("create storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{anekdots: &anekdotRepositoryStub{createErr: errors.New("boom")}})

		if _, err := service.Create(context.Background(), 1, 0, "Очень смешной анекдот"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("like storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{
			anekdots: &anekdotRepositoryStub{byID: &domain.Anekdot{ID: 7}},
			likes:    &reactionRepositoryStub{addErr: errors.New("boom")},
		})

		if err := service.AddLike(context.Background(), 1, 7); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unlike storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{
			anekdots: &anekdotRepositoryStub{byID: &domain.Anekdot{ID: 7}},
			likes:    &reactionRepositoryStub{removeErr: errors.New("boom")},
		})

		if err := service.RemoveLike(context.Background(), 1, 7); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("favorite storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{
			anekdots:  &anekdotRepositoryStub{byID: &domain.Anekdot{ID: 7}},
			favorites: &reactionRepositoryStub{addErr: errors.New("boom")},
		})

		if err := service.AddFavorite(context.Background(), 1, 7); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unfavorite storage error", func(t *testing.T) {
		service := newTestService(serviceDeps{
			anekdots:  &anekdotRepositoryStub{byID: &domain.Anekdot{ID: 7}},
			favorites: &reactionRepositoryStub{removeErr: errors.New("boom")},
		})

		if err := service.RemoveFavorite(context.Background(), 1, 7); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("likes repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{likes: &reactionRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Likes(context.Background(), 1); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("favorites repository error", func(t *testing.T) {
		service := newTestService(serviceDeps{favorites: &reactionRepositoryStub{listErr: errors.New("boom")}})

		if _, err := service.Favorites(context.Background(), 1); err == nil {
			t.Fatal("expected error")
		}
	})
}
