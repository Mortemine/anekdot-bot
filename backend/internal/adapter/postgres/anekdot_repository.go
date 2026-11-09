package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const uniqueViolationCode = "23505"

const anekdotCounters = `
		(SELECT count(*) FROM likes l WHERE l.anekdot_id = a.id)::int AS like_count,
		(SELECT count(*) FROM favorites f WHERE f.anekdot_id = a.id)::int AS favorite_count`

const selectAnekdot = `
	SELECT a.id, a.category_id, c.name, a.text, a.author_id, a.created_at,` + anekdotCounters + `
	FROM anekdots a
	JOIN categories c ON c.id = a.category_id`

const anecdotePageSuffix = `
	LIMIT $2 OFFSET $3`

const anecdoteCategoryFilter = `
	WHERE ($1::int IS NULL OR a.category_id = $1)`

const orderByCreated = `
	ORDER BY a.created_at DESC, a.id DESC`

const orderByLikes = `
	ORDER BY like_count DESC, a.created_at DESC, a.id DESC`

const orderByFavorites = `
	ORDER BY favorite_count DESC, a.created_at DESC, a.id DESC`

var anekdotOrderClauses = map[domain.AnekdotSort]string{
	domain.AnekdotSortCreated:   orderByCreated,
	domain.AnekdotSortLikes:     orderByLikes,
	domain.AnekdotSortFavorites: orderByFavorites,
}

type AnekdotRepository struct {
	pool *pgxpool.Pool
}

func NewAnekdotRepository(pool *pgxpool.Pool) *AnekdotRepository {
	return &AnekdotRepository{pool: pool}
}

func (r *AnekdotRepository) Random(ctx context.Context) (domain.Anekdot, error) {
	return r.find(ctx, "random anecdote", selectAnekdot+` ORDER BY random() LIMIT 1`)
}

func (r *AnekdotRepository) RandomByCategory(ctx context.Context, categoryID int) (domain.Anekdot, error) {
	return r.find(
		ctx,
		fmt.Sprintf("random anecdote in category %d", categoryID),
		selectAnekdot+` WHERE a.category_id = $1 ORDER BY random() LIMIT 1`,
		categoryID,
	)
}

func (r *AnekdotRepository) find(
	ctx context.Context,
	description, query string,
	args ...any,
) (domain.Anekdot, error) {
	anekdot, err := r.scanOne(ctx, query, args...)
	if err == nil {
		return anekdot, nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Anekdot{}, fmt.Errorf("%w: %s", domain.ErrNotFound, description)
	}

	return domain.Anekdot{}, fmt.Errorf("querying %s: %w", description, err)
}

func (r *AnekdotRepository) CountsByCategory(ctx context.Context) (map[int]int, error) {
	return r.categoryTotals(
		ctx,
		`SELECT category_id, count(*) FROM anekdots GROUP BY category_id`,
		"anecdote counts",
	)
}

func (r *AnekdotRepository) PopularityByCategory(ctx context.Context) (map[int]int, error) {
	return r.categoryTotals(ctx, `
		WITH like_counts AS (
		    SELECT a.category_id, count(*)::int AS total
		    FROM likes l
		    JOIN anekdots a ON a.id = l.anekdot_id
		    GROUP BY a.category_id
		), favorite_counts AS (
		    SELECT a.category_id, count(*)::int AS total
		    FROM favorites f
		    JOIN anekdots a ON a.id = f.anekdot_id
		    GROUP BY a.category_id
		)
		SELECT coalesce(l.category_id, f.category_id) AS category_id,
		       coalesce(l.total, 0) + coalesce(f.total, 0) AS popularity
		FROM like_counts l
		FULL OUTER JOIN favorite_counts f ON f.category_id = l.category_id`,
		"category popularity",
	)
}

func (r *AnekdotRepository) categoryTotals(
	ctx context.Context,
	query, description string,
) (map[int]int, error) {
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying %s: %w", description, err)
	}
	defer rows.Close()

	totals := map[int]int{}
	for rows.Next() {
		var (
			categoryID int
			total      int
		)
		if err := rows.Scan(&categoryID, &total); err != nil {
			return nil, fmt.Errorf("scanning %s: %w", description, err)
		}

		totals[categoryID] = total
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating %s: %w", description, err)
	}

	return totals, nil
}

func (r *AnekdotRepository) ByID(ctx context.Context, id int64) (domain.Anekdot, error) {
	return r.find(ctx, fmt.Sprintf("anecdote %d", id), selectAnekdot+` WHERE a.id = $1`, id)
}

func (r *AnekdotRepository) ByText(ctx context.Context, categoryID int, text string) (domain.Anekdot, error) {
	return r.find(
		ctx,
		fmt.Sprintf("anecdote in category %d", categoryID),
		selectAnekdot+` WHERE a.category_id = $1 AND md5(a.text) = md5($2)`,
		categoryID,
		text,
	)
}

func (r *AnekdotRepository) Create(ctx context.Context, anekdot domain.Anekdot) (domain.Anekdot, error) {
	created, err := scanAnekdot(r.pool.QueryRow(
		ctx,
		`WITH inserted AS (
		     INSERT INTO anekdots (category_id, text, author_id)
		     VALUES ($1, $2, $3)
		     RETURNING id, category_id, text, author_id, created_at
		 )
		 SELECT i.id, i.category_id, c.name, i.text, i.author_id, i.created_at, 0 AS like_count,
		     0 AS favorite_count
		 FROM inserted i
		 JOIN categories c ON c.id = i.category_id`,
		anekdot.CategoryID,
		anekdot.Text,
		nullable(anekdot.AuthorID),
	))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Anekdot{}, fmt.Errorf("%w: anecdote already exists", domain.ErrConflict)
		}

		return domain.Anekdot{}, fmt.Errorf("inserting anecdote: %w", err)
	}

	return created, nil
}

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

func (r *AnekdotRepository) List(
	ctx context.Context,
	categoryID *int,
	limit, offset int,
	sort domain.AnekdotSort,
) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(
		ctx,
		selectAnekdot+anecdoteCategoryFilter+orderClause(sort)+anecdotePageSuffix,
		nullable(categoryID),
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("querying anecdotes: %w", err)
	}
	defer rows.Close()

	return collectAnekdots(rows)
}

func (r *AnekdotRepository) Count(ctx context.Context, categoryID *int) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM anekdots a WHERE ($1::int IS NULL OR a.category_id = $1)`,
		nullable(categoryID),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("counting anecdotes: %w", err)
	}

	return total, nil
}

func orderClause(sort domain.AnekdotSort) string {
	if clause, ok := anekdotOrderClauses[sort]; ok {
		return clause
	}

	return orderByCreated
}

func scanAnekdot(row pgx.Row) (domain.Anekdot, error) {
	var (
		anekdot  domain.Anekdot
		authorID pgtype.Int8
	)

	err := row.Scan(
		&anekdot.ID,
		&anekdot.CategoryID,
		&anekdot.CategoryName,
		&anekdot.Text,
		&authorID,
		&anekdot.CreatedAt,
		&anekdot.LikeCount,
		&anekdot.FavoriteCount,
	)
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

func nullable[T any](value *T) any {
	if value == nil {
		return nil
	}

	return *value
}

func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		return ""
	}

	return pgErr.Code
}

func isUniqueViolation(err error) bool {
	return pgErrorCode(err) == uniqueViolationCode
}

func isForeignKeyViolation(err error) bool {
	return pgErrorCode(err) == foreignKeyViolationCode
}
