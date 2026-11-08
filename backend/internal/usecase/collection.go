package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const maxCollectionNameLength = 100

type CollectionService struct {
	collections domain.CollectionRepository
	logger      *slog.Logger
}

func NewCollectionService(collections domain.CollectionRepository, logger *slog.Logger) *CollectionService {
	return &CollectionService{collections: collections, logger: logger}
}

func (s *CollectionService) Mine(
	ctx context.Context,
	userID, anekdotID int64,
) ([]domain.Collection, error) {
	collections, err := s.collections.ListByOwner(ctx, userID, anekdotID)
	if err != nil {
		return nil, fmt.Errorf("listing collections of user %d: %w", userID, err)
	}

	return collections, nil
}

func (s *CollectionService) Public(
	ctx context.Context,
	viewerID, anekdotID int64,
) ([]domain.Collection, error) {
	collections, err := s.collections.ListPublic(ctx, viewerID, anekdotID)
	if err != nil {
		return nil, fmt.Errorf("listing public collections: %w", err)
	}

	return collections, nil
}

func (s *CollectionService) Favorites(ctx context.Context, userID int64) ([]domain.Collection, error) {
	collections, err := s.collections.ListFavorites(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("listing favorite collections of user %d: %w", userID, err)
	}

	return collections, nil
}

func (s *CollectionService) Detail(
	ctx context.Context,
	viewerID, collectionID int64,
) (domain.CollectionDetail, error) {
	collection, err := s.byID(ctx, collectionID, viewerPointer(viewerID))
	if err != nil {
		return domain.CollectionDetail{}, err
	}

	if !collection.IsPublic && collection.OwnerID != viewerID {
		return domain.CollectionDetail{}, fmt.Errorf("%w: collection %d", domain.ErrNotFound, collectionID)
	}

	anekdots, err := s.collections.Anekdots(ctx, collectionID)
	if err != nil {
		return domain.CollectionDetail{}, fmt.Errorf("listing anecdotes of collection %d: %w", collectionID, err)
	}

	return domain.CollectionDetail{Collection: collection, Anekdots: anekdots}, nil
}

func (s *CollectionService) Create(ctx context.Context, userID int64, name string) (domain.Collection, error) {
	normalized, err := normalizeCollectionName(name)
	if err != nil {
		return domain.Collection{}, err
	}

	collection, err := s.collections.Create(ctx, userID, normalized)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("creating collection: %w", err)
	}

	return collection, nil
}

func (s *CollectionService) Update(
	ctx context.Context,
	userID, collectionID int64,
	name string,
	isPublic bool,
) (domain.Collection, error) {
	normalized, err := normalizeCollectionName(name)
	if err != nil {
		return domain.Collection{}, err
	}

	if err := s.owned(ctx, userID, collectionID); err != nil {
		return domain.Collection{}, err
	}

	collection, err := s.collections.Update(ctx, collectionID, userID, normalized, isPublic)
	if err != nil {
		return domain.Collection{}, fmt.Errorf("updating collection %d: %w", collectionID, err)
	}

	return collection, nil
}

func (s *CollectionService) Delete(ctx context.Context, userID, collectionID int64) error {
	if err := s.owned(ctx, userID, collectionID); err != nil {
		return err
	}

	if err := s.collections.Delete(ctx, collectionID, userID); err != nil {
		return fmt.Errorf("deleting collection %d: %w", collectionID, err)
	}

	return nil
}

func (s *CollectionService) AddAnekdot(ctx context.Context, userID, collectionID, anekdotID int64) error {
	if err := s.owned(ctx, userID, collectionID); err != nil {
		return err
	}

	if err := checkAnekdotID(anekdotID); err != nil {
		return err
	}

	if err := s.collections.AddAnekdot(ctx, collectionID, anekdotID); err != nil {
		return fmt.Errorf("adding anecdote %d to collection %d: %w", anekdotID, collectionID, err)
	}

	return nil
}

func (s *CollectionService) RemoveAnekdot(ctx context.Context, userID, collectionID, anekdotID int64) error {
	if err := s.owned(ctx, userID, collectionID); err != nil {
		return err
	}

	if err := checkAnekdotID(anekdotID); err != nil {
		return err
	}

	if err := s.collections.RemoveAnekdot(ctx, collectionID, anekdotID); err != nil {
		return fmt.Errorf("removing anecdote %d from collection %d: %w", anekdotID, collectionID, err)
	}

	return nil
}

func (s *CollectionService) AddFavorite(ctx context.Context, userID, collectionID int64) error {
	collection, err := s.byID(ctx, collectionID, nil)
	if err != nil {
		return err
	}

	if !collection.IsPublic {
		return fmt.Errorf("%w: collection %d is private", domain.ErrForbidden, collectionID)
	}

	if err := s.collections.Favorite(ctx, userID, collectionID); err != nil {
		return fmt.Errorf("favoriting collection %d: %w", collectionID, err)
	}

	return nil
}

func (s *CollectionService) RemoveFavorite(ctx context.Context, userID, collectionID int64) error {
	if err := s.collections.Unfavorite(ctx, userID, collectionID); err != nil {
		return fmt.Errorf("unfavoriting collection %d: %w", collectionID, err)
	}

	return nil
}

func (s *CollectionService) byID(
	ctx context.Context,
	collectionID int64,
	viewerID *int64,
) (domain.Collection, error) {
	if collectionID <= 0 {
		return domain.Collection{}, fmt.Errorf("%w: collection_id must be positive", domain.ErrInvalidInput)
	}

	return s.collections.ByID(ctx, collectionID, viewerID)
}

func (s *CollectionService) owned(ctx context.Context, userID, collectionID int64) error {
	collection, err := s.byID(ctx, collectionID, &userID)
	if err != nil {
		return err
	}

	if collection.OwnerID != userID {
		return fmt.Errorf("%w: collection %d", domain.ErrForbidden, collectionID)
	}

	return nil
}

func normalizeCollectionName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: collection name must not be empty", domain.ErrInvalidInput)
	}

	if utf8.RuneCountInString(trimmed) > maxCollectionNameLength {
		return "", fmt.Errorf("%w: collection name is too long", domain.ErrInvalidInput)
	}

	if strings.ContainsAny(trimmed, "\n\r\t") {
		return "", fmt.Errorf("%w: collection name must be a single line", domain.ErrInvalidInput)
	}

	return trimmed, nil
}

func viewerPointer(viewerID int64) *int64 {
	if viewerID <= 0 {
		return nil
	}

	return &viewerID
}
