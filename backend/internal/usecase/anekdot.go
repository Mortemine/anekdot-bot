// Package usecase implements the application business rules.
package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

// AnekdotService serves category and anecdote use cases.
type AnekdotService struct {
	categories domain.CategoryRepository
	anekdots   domain.AnekdotRepository
	external   domain.ExternalAnekdotSource
	logger     *slog.Logger
}

// NewAnekdotService wires the anecdote use case with its ports.
func NewAnekdotService(
	categories domain.CategoryRepository,
	anekdots domain.AnekdotRepository,
	external domain.ExternalAnekdotSource,
	logger *slog.Logger,
) *AnekdotService {
	return &AnekdotService{
		categories: categories,
		anekdots:   anekdots,
		external:   external,
		logger:     logger,
	}
}

// ListCategories returns every category with its anecdote count.
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

// Random returns a random anecdote, optionally filtered by category.
func (s *AnekdotService) Random(ctx context.Context, categoryID *int) (domain.Anekdot, error) {
	if categoryID == nil {
		anekdot, err := s.anekdots.Random(ctx)
		if err != nil {
			return domain.Anekdot{}, fmt.Errorf("selecting random anecdote: %w", err)
		}

		return anekdot, nil
	}

	// Category 0 exists in the archive as "Вне категорий", so only negative ids are rejected.
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

// External fetches a fresh anecdote from the external provider.
func (s *AnekdotService) External(ctx context.Context) (string, error) {
	text, err := s.external.Random(ctx)
	if err != nil {
		return "", fmt.Errorf("fetching external anecdote: %w", err)
	}

	return text, nil
}
