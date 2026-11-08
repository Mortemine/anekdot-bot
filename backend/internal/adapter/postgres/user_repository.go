package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

// UserRepository stores registered users in PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository returns a PostgreSQL backed user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a user and returns the stored row.
func (r *UserRepository) Create(
	ctx context.Context,
	email, displayName, passwordHash string,
) (domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO users (email, display_name, password_hash)
		 VALUES ($1, $2, $3)
		 RETURNING id, email, display_name, created_at`,
		email,
		displayName,
		passwordHash,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.User{}, fmt.Errorf("%w: email already registered", domain.ErrConflict)
		}

		return domain.User{}, fmt.Errorf("inserting user: %w", err)
	}

	return user, nil
}

// ByEmail returns the credentials of a user by email address.
func (r *UserRepository) ByEmail(ctx context.Context, email string) (domain.Credentials, error) {
	var credentials domain.Credentials

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, email, display_name, created_at, password_hash
		 FROM users
		 WHERE email = $1`,
		email,
	).Scan(
		&credentials.ID,
		&credentials.Email,
		&credentials.DisplayName,
		&credentials.CreatedAt,
		&credentials.PasswordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Credentials{}, fmt.Errorf("%w: user with this email does not exist", domain.ErrNotFound)
		}

		return domain.Credentials{}, fmt.Errorf("querying user by email: %w", err)
	}

	return credentials, nil
}

// ByID returns a user by identifier.
func (r *UserRepository) ByID(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(
		ctx,
		`SELECT id, email, display_name, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%w: user %d", domain.ErrNotFound, id)
		}

		return domain.User{}, fmt.Errorf("querying user %d: %w", id, err)
	}

	return user, nil
}
