package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const foreignKeyViolationCode = "23503"

type reactionQueries struct {
	list   string
	add    string
	remove string
}

type reactionRepository struct {
	pool  *pgxpool.Pool
	noun  string
	query reactionQueries
}

func newReactionRepository(pool *pgxpool.Pool, table, noun string) reactionRepository {
	return reactionRepository{
		pool: pool,
		noun: noun,
		query: reactionQueries{
			list: `
	SELECT a.id, a.category_id, c.name, a.text, a.author_id, a.created_at,` + anekdotCounters + `
	FROM ` + table + ` r
	JOIN anekdots a ON a.id = r.anekdot_id
	JOIN categories c ON c.id = a.category_id
	WHERE r.user_id = $1
	ORDER BY r.created_at DESC, a.id DESC`,
			add:    `INSERT INTO ` + table + ` (user_id, anekdot_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			remove: `DELETE FROM ` + table + ` WHERE user_id = $1 AND anekdot_id = $2`,
		},
	}
}

func (r reactionRepository) list(ctx context.Context, userID int64) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(ctx, r.query.list, userID)
	if err != nil {
		return nil, fmt.Errorf("querying %ss of user %d: %w", r.noun, userID, err)
	}
	defer rows.Close()

	return collectAnekdots(rows)
}

func (r reactionRepository) add(ctx context.Context, userID, anekdotID int64) error {
	if _, err := r.pool.Exec(ctx, r.query.add, userID, anekdotID); err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: anecdote %d", domain.ErrNotFound, anekdotID)
		}

		return fmt.Errorf("inserting %s: %w", r.noun, err)
	}

	return nil
}

func (r reactionRepository) remove(ctx context.Context, userID, anekdotID int64) error {
	if _, err := r.pool.Exec(ctx, r.query.remove, userID, anekdotID); err != nil {
		return fmt.Errorf("deleting %s: %w", r.noun, err)
	}

	return nil
}
