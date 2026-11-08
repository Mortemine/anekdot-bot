package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

// SessionRepository stores issued sessions in PostgreSQL.
type SessionRepository struct {
	pool *pgxpool.Pool
}

// NewSessionRepository returns a PostgreSQL backed session repository.
func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

// Create stores a new session.
func (r *SessionRepository) Create(ctx context.Context, session domain.Session) error {
	_, err := r.pool.Exec(
		ctx,
		`INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		session.TokenHash,
		session.UserID,
		session.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("inserting session: %w", err)
	}

	return nil
}

// ByTokenHash resolves a live session to its user.
func (r *SessionRepository) ByTokenHash(ctx context.Context, tokenHash string) (domain.User, error) {
	var user domain.User

	err := r.pool.QueryRow(
		ctx,
		`SELECT u.id, u.email, u.display_name, u.created_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > now()`,
		tokenHash,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, fmt.Errorf("%w: session is unknown or expired", domain.ErrNotFound)
		}

		return domain.User{}, fmt.Errorf("querying session: %w", err)
	}

	return user, nil
}

// Delete removes one session.
func (r *SessionRepository) Delete(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash)
	if err != nil {
		return fmt.Errorf("deleting session: %w", err)
	}

	return nil
}
