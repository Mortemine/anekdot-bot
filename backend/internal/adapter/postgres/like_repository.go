package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const foreignKeyViolationCode = "23503"

const selectLikeAnekdot = `
	SELECT a.id, a.category_id, c.name, a.text, a.author_id, a.created_at,` + anekdotLikeCount + `
	FROM likes l
	JOIN anekdots a ON a.id = l.anekdot_id
	JOIN categories c ON c.id = a.category_id
	WHERE l.user_id = $1
	ORDER BY l.created_at DESC, a.id DESC`

type LikeRepository struct {
	pool *pgxpool.Pool
}

func NewLikeRepository(pool *pgxpool.Pool) *LikeRepository {
	return &LikeRepository{pool: pool}
}

func (r *LikeRepository) List(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(ctx, selectLikeAnekdot, userID)
	if err != nil {
		return nil, fmt.Errorf("querying likes of user %d: %w", userID, err)
	}
	defer rows.Close()

	return collectAnekdots(rows)
}

func (r *LikeRepository) Add(ctx context.Context, userID, anekdotID int64) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO likes (user_id, anekdot_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID,
		anekdotID,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: anecdote %d", domain.ErrNotFound, anekdotID)
		}

		return fmt.Errorf("inserting like: %w", err)
	}

	return nil
}

func (r *LikeRepository) Remove(ctx context.Context, userID, anekdotID int64) error {
	_, err := r.pool.Exec(
		ctx,
		`DELETE FROM likes WHERE user_id = $1 AND anekdot_id = $2`,
		userID,
		anekdotID,
	)
	if err != nil {
		return fmt.Errorf("deleting like: %w", err)
	}

	return nil
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolationCode
}
