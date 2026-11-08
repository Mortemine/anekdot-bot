package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

const selectCollection = `
	SELECT c.id, c.owner_id, u.display_name, c.name, c.is_public, c.created_at, c.updated_at,
		(SELECT count(*) FROM collection_anekdots ca WHERE ca.collection_id = c.id)::int AS anekdot_count,
		EXISTS (
			SELECT 1 FROM collection_anekdots cm
			WHERE cm.collection_id = c.id AND cm.anekdot_id = $1
		) AS contains_anekdot,
		EXISTS (
			SELECT 1 FROM collection_favorites cf
			WHERE cf.collection_id = c.id AND cf.user_id = $2
		) AS favorited
	FROM collections c
	JOIN users u ON u.id = c.owner_id`

const collectionOrder = `
	ORDER BY c.created_at DESC, c.id DESC`

const selectCollectionAnekdots = selectAnekdot + `
	JOIN collection_anekdots ca ON ca.anekdot_id = a.id
	WHERE ca.collection_id = $1
	ORDER BY ca.created_at DESC, a.id DESC`

type CollectionRepository struct {
	pool *pgxpool.Pool
}

func NewCollectionRepository(pool *pgxpool.Pool) *CollectionRepository {
	return &CollectionRepository{pool: pool}
}

func (r *CollectionRepository) ListByOwner(
	ctx context.Context,
	ownerID, anekdotID int64,
) ([]domain.Collection, error) {
	return r.list(ctx, selectCollection+`
	WHERE c.owner_id = $3`+collectionOrder, optionalID(anekdotID), ownerID, ownerID)
}

func (r *CollectionRepository) ListPublic(
	ctx context.Context,
	viewerID, anekdotID int64,
) ([]domain.Collection, error) {
	return r.list(ctx, selectCollection+`
	WHERE ($3::bigint IS NULL OR c.owner_id = $3) AND c.is_public`+collectionOrder,
		optionalID(anekdotID), optionalID(viewerID), nil)
}

func (r *CollectionRepository) ListFavorites(ctx context.Context, userID int64) ([]domain.Collection, error) {
	return r.list(ctx, selectCollection+`
	WHERE ($3::bigint IS NULL OR c.owner_id = $3)
		AND EXISTS (
			SELECT 1 FROM collection_favorites cf2
			WHERE cf2.collection_id = c.id AND cf2.user_id = $2
		)`+collectionOrder, nil, userID, nil)
}

func (r *CollectionRepository) ByID(
	ctx context.Context,
	id int64,
	viewerID *int64,
) (domain.Collection, error) {
	collection, err := r.scanOne(ctx, selectCollection+`
	WHERE c.id = $3`, nil, nullableInt64(viewerID), id)
	if err != nil {
		return domain.Collection{}, err
	}

	return collection, nil
}

func (r *CollectionRepository) Create(ctx context.Context, ownerID int64, name string) (domain.Collection, error) {
	var id int64

	err := r.pool.QueryRow(ctx, `
		INSERT INTO collections (owner_id, name)
		VALUES ($1, $2)
		RETURNING id`, ownerID, name).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Collection{}, fmt.Errorf("%w: collection %q already exists", domain.ErrConflict, name)
		}

		if isForeignKeyViolation(err) {
			return domain.Collection{}, fmt.Errorf("%w: user %d", domain.ErrNotFound, ownerID)
		}

		return domain.Collection{}, fmt.Errorf("creating collection: %w", err)
	}

	return r.ByID(ctx, id, &ownerID)
}

func (r *CollectionRepository) Update(
	ctx context.Context,
	id, ownerID int64,
	name string,
	isPublic bool,
) (domain.Collection, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE collections
		SET name = $1, is_public = $2, updated_at = now()
		WHERE id = $3 AND owner_id = $4`, name, isPublic, id, ownerID)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Collection{}, fmt.Errorf("%w: collection %q already exists", domain.ErrConflict, name)
		}

		return domain.Collection{}, fmt.Errorf("updating collection %d: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.Collection{}, fmt.Errorf("%w: collection %d", domain.ErrNotFound, id)
	}

	return r.ByID(ctx, id, &ownerID)
}

func (r *CollectionRepository) Delete(ctx context.Context, id, ownerID int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM collections WHERE id = $1 AND owner_id = $2`, id, ownerID)
	if err != nil {
		return fmt.Errorf("deleting collection %d: %w", id, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: collection %d", domain.ErrNotFound, id)
	}

	return nil
}

func (r *CollectionRepository) AddAnekdot(ctx context.Context, collectionID, anekdotID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collection_anekdots (collection_id, anekdot_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, collectionID, anekdotID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf(
				"%w: collection %d or anecdote %d",
				domain.ErrNotFound,
				collectionID,
				anekdotID,
			)
		}

		return fmt.Errorf("adding anecdote %d to collection %d: %w", anekdotID, collectionID, err)
	}

	return nil
}

func (r *CollectionRepository) RemoveAnekdot(ctx context.Context, collectionID, anekdotID int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM collection_anekdots
		WHERE collection_id = $1 AND anekdot_id = $2`, collectionID, anekdotID)
	if err != nil {
		return fmt.Errorf("removing anecdote %d from collection %d: %w", anekdotID, collectionID, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: anecdote %d in collection %d", domain.ErrNotFound, anekdotID, collectionID)
	}

	return nil
}

func (r *CollectionRepository) Anekdots(ctx context.Context, collectionID int64) ([]domain.Anekdot, error) {
	rows, err := r.pool.Query(ctx, selectCollectionAnekdots, collectionID)
	if err != nil {
		return nil, fmt.Errorf("querying anecdotes of collection %d: %w", collectionID, err)
	}
	defer rows.Close()

	anekdots, err := collectAnekdots(rows)
	if err != nil {
		return nil, err
	}

	return anekdots, nil
}

func (r *CollectionRepository) Favorite(ctx context.Context, userID, collectionID int64) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO collection_favorites (user_id, collection_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, collectionID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("%w: collection %d", domain.ErrNotFound, collectionID)
		}

		return fmt.Errorf("favoriting collection %d: %w", collectionID, err)
	}

	return nil
}

func (r *CollectionRepository) Unfavorite(ctx context.Context, userID, collectionID int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM collection_favorites
		WHERE user_id = $1 AND collection_id = $2`, userID, collectionID)
	if err != nil {
		return fmt.Errorf("unfavoriting collection %d: %w", collectionID, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: favorite of collection %d", domain.ErrNotFound, collectionID)
	}

	return nil
}

func (r *CollectionRepository) list(
	ctx context.Context,
	query string,
	args ...any,
) ([]domain.Collection, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying collections: %w", err)
	}
	defer rows.Close()

	collections := []domain.Collection{}

	for rows.Next() {
		collection, err := scanCollection(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning collection: %w", err)
		}

		collections = append(collections, collection)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating collections: %w", err)
	}

	return collections, nil
}

func (r *CollectionRepository) scanOne(ctx context.Context, query string, args ...any) (domain.Collection, error) {
	collection, err := scanCollection(r.pool.QueryRow(ctx, query, args...))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Collection{}, fmt.Errorf("%w: collection", domain.ErrNotFound)
		}

		return domain.Collection{}, fmt.Errorf("querying collection: %w", err)
	}

	return collection, nil
}

func scanCollection(row pgx.Row) (domain.Collection, error) {
	var collection domain.Collection

	err := row.Scan(
		&collection.ID,
		&collection.OwnerID,
		&collection.OwnerName,
		&collection.Name,
		&collection.IsPublic,
		&collection.CreatedAt,
		&collection.UpdatedAt,
		&collection.AnekdotCount,
		&collection.ContainsAnekdot,
		&collection.Favorited,
	)
	if err != nil {
		return domain.Collection{}, err
	}

	return collection, nil
}

func optionalID(value int64) any {
	if value <= 0 {
		return nil
	}

	return value
}
