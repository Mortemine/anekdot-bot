package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type CategoryRepository struct {
	pool *pgxpool.Pool
}

func NewCategoryRepository(pool *pgxpool.Pool) *CategoryRepository {
	return &CategoryRepository{pool: pool}
}

func (r *CategoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name FROM categories ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying categories: %w", err)
	}
	defer rows.Close()

	categories := []domain.Category{}
	for rows.Next() {
		var category domain.Category
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}

		categories = append(categories, category)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating categories: %w", err)
	}

	return categories, nil
}

func (r *CategoryRepository) Get(ctx context.Context, id int) (domain.Category, error) {
	var category domain.Category

	err := r.pool.QueryRow(ctx, `SELECT id, name FROM categories WHERE id = $1`, id).
		Scan(&category.ID, &category.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, fmt.Errorf("%w: category %d", domain.ErrNotFound, id)
		}

		return domain.Category{}, fmt.Errorf("querying category %d: %w", id, err)
	}

	return category, nil
}
