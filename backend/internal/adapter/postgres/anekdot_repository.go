package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

// uniqueViolationCode is PostgreSQL's SQLSTATE for a unique index conflict.
const uniqueViolationCode = "23505"

const selectAnekdot = `
	SELECT a.id, a.category_id, c.name, a.text, a.author_id, a.created_at
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

// ByID returns a single anecdote.
func (r *AnekdotRepository) ByID(ctx context.Context, id int64) (domain.Anekdot, error) {
	anekdot, err := r.scanOne(ctx, selectAnekdot+` WHERE a.id = $1`, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Anekdot{}, fmt.Errorf("%w: anecdote %d", domain.ErrNotFound, id)
		}

		return domain.Anekdot{}, fmt.Errorf("querying anecdote %d: %w", id, err)
	}

	return anekdot, nil
}

// Create stores a new anecdote and returns the stored row.
func (r *AnekdotRepository) Create(ctx context.Context, anekdot domain.Anekdot) (domain.Anekdot, error) {
	created, err := scanAnekdot(r.pool.QueryRow(
		ctx,
		`WITH inserted AS (
		     INSERT INTO anekdots (category_id, text, author_id)
		     VALUES ($1, $2, $3)
		     RETURNING id, category_id, text, author_id, created_at
		 )
		 SELECT i.id, i.category_id, c.name, i.text, i.author_id, i.created_at
		 FROM inserted i
		 JOIN categories c ON c.id = i.category_id`,
		anekdot.CategoryID,
		anekdot.Text,
		nullableInt64(anekdot.AuthorID),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Anekdot{}, fmt.Errorf("%w: anecdote already exists", domain.ErrConflict)
		}

		return domain.Anekdot{}, fmt.Errorf("inserting anecdote: %w", err)
	}

	return created, nil
}

// ListByAuthor returns the anecdotes submitted by one user, newest first.
func (r *AnekdotRepository) ListByAuthor(ctx context.Context, authorID int64, limit int) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(
		ctx,
		selectAnekdot+` WHERE a.author_id = $1 ORDER BY a.created_at DESC, a.id DESC LIMIT $2`,
		authorID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("querying anecdotes of author %d: %w", authorID, err)
	}
	defer rows.Close()

	return collectAnekdots(rows)
}

func (r *AnekdotRepository) scanOne(ctx context.Context, query string, args ...any) (domain.Anekdot, error) {
	return scanAnekdot(r.pool.QueryRow(ctx, query, args...))
}

func scanAnekdot(row pgx.Row) (domain.Anekdot, error) {
	var (
		anekdot  domain.Anekdot
		authorID pgtype.Int8
	)

	err := row.Scan(&anekdot.ID, &anekdot.CategoryID, &anekdot.CategoryName, &anekdot.Text, &authorID, &anekdot.CreatedAt)
	if err != nil {
		return domain.Anekdot{}, err
	}

	anekdot.AuthorID = int64Pointer(authorID)

	return anekdot, nil
}

func collectAnekdots(rows pgx.Rows) ([]domain.Anekdot, error) {
	anekdots := []domain.Anekdot{}
	for rows.Next() {
		anekdot, err := scanAnekdot(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning anecdote: %w", err)
		}

		anekdots = append(anekdots, anekdot)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating anecdotes: %w", err)
	}

	return anekdots, nil
}

func int64Pointer(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}

	number := value.Int64

	return &number
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}

	return *value
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode
}
