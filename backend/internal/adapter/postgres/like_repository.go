package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type LikeRepository struct {
	reactions reactionRepository
}

func NewLikeRepository(pool *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{reactions: newReactionRepository(pool, "likes", "like")}
}

func (r *LikeRepository) List(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	return r.reactions.list(ctx, userID)
}

func (r *LikeRepository) Add(ctx context.Context, userID, anekdotID int64) error {
	return r.reactions.add(ctx, userID, anekdotID)
}

func (r *LikeRepository) Remove(ctx context.Context, userID, anekdotID int64) error {
	return r.reactions.remove(ctx, userID, anekdotID)
}
