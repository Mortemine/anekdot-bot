package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const (
	minAnekdotLength    = 10
	maxAnekdotLength    = 4000
	MineAnekdotLimit    = 200
	categoryPageSize    = 40
	maxCategoryPageSize = 100
	externalCategoryID  = 46
)

type AnekdotService struct {
	categories domain.CategoryRepository
	anekdots   domain.AnekdotRepository
	likes      domain.LikeRepository
	favorites  domain.FavoriteRepository
	external   domain.ExternalAnekdotSource
	logger     *slog.Logger
}

func NewAnekdotService(
	categories domain.CategoryRepository,
	anekdots domain.AnekdotRepository,
	likes domain.LikeRepository,
	favorites domain.FavoriteRepository,
	external domain.ExternalAnekdotSource,
	logger *slog.Logger,
) *AnekdotService {
	return &AnekdotService{
		categories: categories,
		anekdots:   anekdots,
		likes:      likes,
		favorites:  favorites,
		external:   external,
		logger:     logger,
	}
}

func (s *AnekdotService) ListCategories(ctx context.Context) ([]domain.CategorySummary, error) {
	categories, err := s.categories.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	counts, err := s.anekdots.CountsByCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting anecdotes: %w", err)
	}

	popularity, err := s.anekdots.PopularityByCategory(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting category popularity: %w", err)
	}

	summaries := make([]domain.CategorySummary, 0, len(categories))
	for _, category := range categories {
		summaries = append(summaries, domain.CategorySummary{
			Category:     category,
			AnekdotCount: counts[category.ID],
			Popularity:   popularity[category.ID],
		})
	}

	return summaries, nil
}

func (s *AnekdotService) Anekdots(
	ctx context.Context,
	categoryID *int,
	limit, offset int,
	sort domain.AnekdotSort,
) (domain.AnekdotPage, error) {
	normalizedSort, err := domain.ParseAnekdotSort(string(sort))
	if err != nil {
		return domain.AnekdotPage{}, err
	}

	if offset < 0 {
		return domain.AnekdotPage{}, fmt.Errorf("%w: offset must not be negative", domain.ErrInvalidInput)
	}

	limit = clampPageSize(limit)

	if categoryID != nil {
		if err := s.checkCategory(ctx, *categoryID); err != nil {
			return domain.AnekdotPage{}, err
		}
	}

	anekdots, err := s.anekdots.List(ctx, categoryID, limit, offset, normalizedSort)
	if err != nil {
		return domain.AnekdotPage{}, fmt.Errorf("listing anecdotes: %w", err)
	}

	total, err := s.anekdots.Count(ctx, categoryID)
	if err != nil {
		return domain.AnekdotPage{}, fmt.Errorf("counting anecdotes: %w", err)
	}

	return domain.AnekdotPage{Anekdots: anekdots, Total: total, Limit: limit, Offset: offset}, nil
}

func clampPageSize(limit int) int {
	if limit <= 0 {
		return categoryPageSize
	}

	return min(limit, maxCategoryPageSize)
}

func normalizeAnekdotText(text string) (string, error) {
	trimmed := strings.TrimSpace(text)

	length := utf8.RuneCountInString(trimmed)
	if length < minAnekdotLength || length > maxAnekdotLength {
		return "", fmt.Errorf(
			"%w: anecdote text must be %d to %d characters long",
			domain.ErrInvalidInput,
			minAnekdotLength,
			maxAnekdotLength,
		)
	}

	return trimmed, nil
}

func (s *AnekdotService) Random(ctx context.Context, categoryID *int) (domain.Anekdot, error) {
	if categoryID == nil {
		anekdot, err := s.anekdots.Random(ctx)
		if err != nil {
			return domain.Anekdot{}, fmt.Errorf("selecting random anecdote: %w", err)
		}

		return anekdot, nil
	}

	if err := s.checkCategory(ctx, *categoryID); err != nil {
		return domain.Anekdot{}, err
	}

	anekdot, err := s.anekdots.RandomByCategory(ctx, *categoryID)
	if err != nil {
		return domain.Anekdot{}, fmt.Errorf("selecting random anecdote in category %d: %w", *categoryID, err)
	}

	return anekdot, nil
}

func (s *AnekdotService) External(ctx context.Context) (string, error) {
	text, err := s.external.Random(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching external anecdote: %w", err)
	}

	return text, nil
}

func (s *AnekdotService) SaveExternal(
	ctx context.Context,
	userID int64,
	text string,
) (domain.Anekdot, error) {
	trimmed, err := normalizeAnekdotText(text)
	if err != nil {
		return domain.Anekdot{}, err
	}

	saved, err := s.anekdots.Create(ctx, domain.Anekdot{
		CategoryID: externalCategoryID,
		Text:       trimmed,
	})
	if err != nil {
		if !errors.Is(err, domain.ErrConflict) {
			return domain.Anekdot{}, fmt.Errorf("storing external anecdote: %w", err)
		}

		saved, err = s.anekdots.ByText(ctx, externalCategoryID, trimmed)
		if err != nil {
			return domain.Anekdot{}, fmt.Errorf("loading existing external anecdote: %w", err)
		}
	}

	if err := s.likes.Add(ctx, userID, saved.ID); err != nil {
		return domain.Anekdot{}, fmt.Errorf("saving like: %w", err)
	}

	s.logger.InfoContext(ctx, "external anecdote saved", "anekdot_id", saved.ID, "user_id", userID)

	return saved, nil
}

func (s *AnekdotService) Create(
	ctx context.Context,
	authorID int64,
	categoryID int,
	text string,
) (domain.Anekdot, error) {
	trimmed, err := normalizeAnekdotText(text)
	if err != nil {
		return domain.Anekdot{}, err
	}

	if err := s.checkCategory(ctx, categoryID); err != nil {
		return domain.Anekdot{}, err
	}

	author := authorID

	created, err := s.anekdots.Create(ctx, domain.Anekdot{
		CategoryID: categoryID,
		Text:       trimmed,
		AuthorID:   &author,
	})
	if err != nil {
		if errors.Is(err, domain.ErrConflict) {
			return domain.Anekdot{}, fmt.Errorf("%w: duplicate text in category %d", domain.ErrConflict, categoryID)
		}

		return domain.Anekdot{}, fmt.Errorf("storing anecdote: %w", err)
	}

	s.logger.InfoContext(ctx, "anecdote submitted", "anekdot_id", created.ID, "author_id", authorID)

	return created, nil
}

func (s *AnekdotService) Mine(ctx context.Context, authorID int64) ([]domain.Anekdot, error) {
	anekdots, err := s.anekdots.ListByAuthor(ctx, authorID, MineAnekdotLimit)
	if err != nil {
		return nil, fmt.Errorf("listing anecdotes of author %d: %w", authorID, err)
	}

	return anekdots, nil
}

func (s *AnekdotService) Likes(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	likes, err := s.likes.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing likes of user %d: %w", userID, err)
	}

	return likes, nil
}

func (s *AnekdotService) AddLike(ctx context.Context, userID, anekdotID int64) error {
	return s.addReaction(ctx, userID, anekdotID, "like", s.likes.Add)
}

func (s *AnekdotService) RemoveLike(ctx context.Context, userID, anekdotID int64) error {
	return s.removeReaction(ctx, userID, anekdotID, "like", s.likes.Remove)
}

func (s *AnekdotService) Favorites(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	favorites, err := s.favorites.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing favorites of user %d: %w", userID, err)
	}

	return favorites, nil
}

func (s *AnekdotService) AddFavorite(ctx context.Context, userID, anekdotID int64) error {
	return s.addReaction(ctx, userID, anekdotID, "favorite", s.favorites.Add)
}

func (s *AnekdotService) RemoveFavorite(ctx context.Context, userID, anekdotID int64) error {
	return s.removeReaction(ctx, userID, anekdotID, "favorite", s.favorites.Remove)
}

func (s *AnekdotService) addReaction(
	ctx context.Context,
	userID, anekdotID int64,
	noun string,
	add func(ctx context.Context, userID, anekdotID int64) error,
) error {
	if err := checkAnekdotID(anekdotID); err != nil {
		return err
	}

	if _, err := s.anekdots.ByID(ctx, anekdotID); err != nil {
		return err
	}

	if err := add(ctx, userID, anekdotID); err != nil {
		return fmt.Errorf("saving %s: %w", noun, err)
	}

	return nil
}

func (s *AnekdotService) removeReaction(
	ctx context.Context,
	userID, anekdotID int64,
	noun string,
	remove func(ctx context.Context, userID, anekdotID int64) error,
) error {
	if err := checkAnekdotID(anekdotID); err != nil {
		return err
	}

	if err := remove(ctx, userID, anekdotID); err != nil {
		return fmt.Errorf("removing %s: %w", noun, err)
	}

	return nil
}

func checkAnekdotID(anekdotID int64) error {
	if anekdotID <= 0 {
		return fmt.Errorf("%w: anekdot_id must be positive", domain.ErrInvalidInput)
	}

	return nil
}

func (s *AnekdotService) checkCategory(ctx context.Context, categoryID int) error {
	if categoryID < 0 {
		return fmt.Errorf("%w: category_id must not be negative", domain.ErrInvalidInput)
	}

	_, err := s.categories.Get(ctx, categoryID)

	return err
}
