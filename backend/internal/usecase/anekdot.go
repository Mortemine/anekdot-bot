package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const (
	minAnekdotLength = 10
	maxAnekdotLength = 4000
	userAnekdotLimit = 200
)

type AnekdotService struct {
	categories domain.CategoryRepository
	anekdots   domain.AnekdotRepository
	favorites  domain.FavoriteRepository
	external   domain.ExternalAnekdotSource
	logger     *slog.Logger
}

func NewAnekdotService(
	categories domain.CategoryRepository,
	anekdots domain.AnekdotRepository,
	favorites domain.FavoriteRepository,
	external domain.ExternalAnekdotSource,
	logger *slog.Logger,
) *AnekdotService {
	return &AnekdotService{
		categories: categories,
		anekdots:   anekdots,
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

	summaries := make([]domain.CategorySummary, 0, len(categories))
	for _, category := range categories {
		summaries = append(summaries, domain.CategorySummary{
			Category:     category,
			AnekdotCount: counts[category.ID],
		})
	}

	return summaries, nil
}

func (s *AnekdotService) Random(ctx context.Context, categoryID *int) (domain.Anekdot, error) {
	if categoryID == nil {
		anekdot, err := s.anekdots.Random(ctx)
		if err != nil {
			return domain.Anekdot{}, fmt.Errorf("selecting random anecdote: %w", err)
		}

		return anekdot, nil
	}

	if *categoryID < 0 {
		return domain.Anekdot{}, fmt.Errorf("%w: category_id must not be negative", domain.ErrInvalidInput)
	}

	if _, err := s.categories.Get(ctx, *categoryID); err != nil {
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

func (s *AnekdotService) Create(
	ctx context.Context,
	authorID int64,
	categoryID int,
	text string,
) (domain.Anekdot, error) {
	trimmed := strings.TrimSpace(text)

	length := utf8.RuneCountInString(trimmed)
	if length < minAnekdotLength || length > maxAnekdotLength {
		return domain.Anekdot{}, fmt.Errorf(
			"%w: anecdote text must be %d to %d characters long",
			domain.ErrInvalidInput,
			minAnekdotLength,
			maxAnekdotLength,
		)
	}

	if categoryID < 0 {
		return domain.Anekdot{}, fmt.Errorf("%w: category_id must not be negative", domain.ErrInvalidInput)
	}

	if _, err := s.categories.Get(ctx, categoryID); err != nil {
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
	anekdots, err := s.anekdots.ListByAuthor(ctx, authorID, userAnekdotLimit)
	if err != nil {
		return nil, fmt.Errorf("listing anecdotes of author %d: %w", authorID, err)
	}

	return anekdots, nil
}

func (s *AnekdotService) Favorites(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	favorites, err := s.favorites.List(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing favorites of user %d: %w", userID, err)
	}

	return favorites, nil
}

func (s *AnekdotService) AddFavorite(ctx context.Context, userID, anekdotID int64) error {
	if anekdotID <= 0 {
		return fmt.Errorf("%w: anekdot_id must be positive", domain.ErrInvalidInput)
	}

	if _, err := s.anekdots.ByID(ctx, anekdotID); err != nil {
		return err
	}

	if err := s.favorites.Add(ctx, userID, anekdotID); err != nil {
		return fmt.Errorf("saving favorite: %w", err)
	}

	return nil
}

func (s *AnekdotService) RemoveFavorite(ctx context.Context, userID, anekdotID int64) error {
	if anekdotID <= 0 {
		return fmt.Errorf("%w: anekdot_id must be positive", domain.ErrInvalidInput)
	}

	if err := s.favorites.Remove(ctx, userID, anekdotID); err != nil {
		return fmt.Errorf("removing favorite: %w", err)
	}

	return nil
}
