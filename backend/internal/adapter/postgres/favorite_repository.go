package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type FavoriteRepository struct {
	reactions reactionRepository
}

func NewFavoriteRepository(pool *pgxpool.Pool) *FavoriteRepository {
	return &FavoriteRepository{reactions: newReactionRepository(pool, "favorites", "favorite")}
}

func (r *FavoriteRepository) List(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	return r.reactions.list(ctx, userID)
}

func (r *FavoriteRepository) Add(ctx context.Context, userID, anekdotID int64) error {
	return r.reactions.add(ctx, userID, anekdotID)
}

func (r *FavoriteRepository) Remove(ctx context.Context, userID, anekdotID int64) error {
	return r.reactions.remove(ctx, userID, anekdotID)
}
