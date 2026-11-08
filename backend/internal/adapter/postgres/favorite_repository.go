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

const selectFavoriteAnekdot = `
	SELECT a.id, a.category_id, c.name, a.text, a.author_id, a.created_at
	FROM favorites f
	JOIN anekdots a ON a.id = f.anekdot_id
	JOIN categories c ON c.id = a.category_id
	WHERE f.user_id = $1
	ORDER BY f.created_at DESC, a.id DESC`

type FavoriteRepository struct {
	pool *pgxpool.Pool
}

func NewFavoriteRepository(pool *pgxpool.Pool) *FavoriteRepository {
	return &FavoriteRepository{pool: pool}
}

func (r *FavoriteRepository) List(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(ctx, selectFavoriteAnekdot, userID)
	if err != nil {
		return nil, fmt.Errorf("querying favorites of user %d: %w", userID, err)
	}
	defer rows.Close()

	return collectAnekdots(rows)
}

func (r *FavoriteRepository) Add(ctx context.Context, userID, anekdotID int64) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO favorites (user_id, anekdot_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID,
		anekdotID,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: anecdote %d", domain.ErrNotFound, anekdotID)
		}

		return fmt.Errorf("inserting favorite: %w", err)
	}

	return nil
}

func (r *FavoriteRepository) Remove(ctx context.Context, userID, anekdotID int64) error {
	_, err := r.pool.Exec(
		ctx,
		`DELETE FROM favorites WHERE user_id = $1 AND anekdot_id = $2`,
		userID,
		anekdotID,
	)
	if err != nil {
		return fmt.Errorf("deleting favorite: %w", err)
	}

	return nil
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == foreignKeyViolationCode
}
