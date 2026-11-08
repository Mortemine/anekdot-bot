package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const selectAnekdot = `
	SELECT a.id, a.category_id, c.name, a.text, a.created_at
	FROM anekdots a
	JOIN categories c ON c.id = a.category_id`

// AnekdotRepository stores anecdotes in PostgreSQL.
type AnekdotRepository struct {
	pool *pgxpool.Pool
}

// NewAnekdotRepository returns a PostgreSQL backed anecdote repository.
func NewAnekdotRepository(pool *pgxpool.Pool) *AnekdotRepository {
	return &AnekdotRepository{pool: pool}
}

// Random returns a random anecdote from the whole table.
func (r *AnekdotRepository) Random(ctx context.Context) (domain.Anekdot, error) {
	anekdot, err := r.scanOne(ctx, selectAnekdot+` ORDER BY random() LIMIT 1`)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Anekdot{}, fmt.Errorf("%w: no anecdotes in database", domain.ErrNotFound)
		}

		return domain.Anekdot{}, fmt.Errorf("querying random anecdote: %w", err)
	}

	return anekdot, nil
}

// RandomByCategory returns a random anecdote from a single category.
func (r *AnekdotRepository) RandomByCategory(ctx context.Context, categoryID int) (domain.Anekdot, error) {
	anekdot, err := r.scanOne(
		ctx,
		selectAnekdot+` WHERE a.category_id = $1 ORDER BY random() LIMIT 1`,
		categoryID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Anekdot{}, fmt.Errorf("%w: no anecdotes in category %d", domain.ErrNotFound, categoryID)
		}

		return domain.Anekdot{}, fmt.Errorf("querying random anecdote in category %d: %w", categoryID, err)
	}

	return anekdot, nil
}

// CountsByCategory returns the number of anecdotes per category id.
func (r *AnekdotRepository) CountsByCategory(ctx context.Context) (map[int]int, error) {
	rows, err := r.pool.Query(ctx, `SELECT category_id, count(*) FROM anekdots GROUP BY category_id`)
	if err != nil {
		return nil, fmt.Errorf("querying anecdote counts: %w", err)
	}
	defer rows.Close()

	counts := map[int]int{}
	for rows.Next() {
		var (
			categoryID int
			count      int
		)
		if err := rows.Scan(&categoryID, &count); err != nil {
			return nil, fmt.Errorf("scanning anecdote count: %w", err)
		}

		counts[categoryID] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating anecdote counts: %w", err)
	}

	return counts, nil
}

func (r *AnekdotRepository) scanOne(ctx context.Context, query string, args ...any) (domain.Anekdot, error) {
	var anekdot domain.Anekdot

	err := r.pool.QueryRow(ctx, query, args...).
		Scan(&anekdot.ID, &anekdot.CategoryID, &anekdot.CategoryName, &anekdot.Text, &anekdot.CreatedAt)

	return anekdot, err
}
