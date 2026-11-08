package domain

import "context"

// CategoryRepository is the driven port for category storage.
type CategoryRepository interface {
	List(ctx context.Context) ([]Category, error)
	Get(ctx context.Context, id int) (Category, error)
}

// AnekdotRepository is the driven port for anecdote storage.
type AnekdotRepository interface {
	Random(ctx context.Context) (Anekdot, error)
	RandomByCategory(ctx context.Context, categoryID int) (Anekdot, error)
	CountsByCategory(ctx context.Context) (map[int]int, error)
	// ByID returns a single anecdote; a missing row yields ErrNotFound.
	ByID(ctx context.Context, id int64) (Anekdot, error)
	// Create stores a new anecdote. A duplicate text inside the same category
	// yields ErrConflict.
	Create(ctx context.Context, anekdot Anekdot) (Anekdot, error)
	// ListByAuthor returns the anecdotes submitted by one user, newest first.
	ListByAuthor(ctx context.Context, authorID int64, limit int) ([]Anekdot, error)
}

// UserRepository is the driven port for registered users.
type UserRepository interface {
	// Create inserts a user. A duplicate email yields ErrConflict.
	Create(ctx context.Context, email, displayName, passwordHash string) (User, error)
	// ByEmail returns the credentials of a user; a missing user yields
	// ErrNotFound.
	ByEmail(ctx context.Context, email string) (Credentials, error)
	// ByID returns a user; a missing user yields ErrNotFound.
	ByID(ctx context.Context, id int64) (User, error)
}

// SessionRepository is the driven port for issued sessions.
type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	// ByTokenHash resolves a live session to its user; an unknown or expired
	// token yields ErrNotFound.
	ByTokenHash(ctx context.Context, tokenHash string) (User, error)
	// Delete removes one session. Deleting an absent token is not an error.
	Delete(ctx context.Context, tokenHash string) error
}

// FavoriteRepository is the driven port for saved anecdotes.
type FavoriteRepository interface {
	// List returns the saved anecdotes, most recently saved first.
	List(ctx context.Context, userID int64) ([]Anekdot, error)
	// Add is idempotent: saving the same anecdote twice keeps one row.
	Add(ctx context.Context, userID, anekdotID int64) error
	// Remove is idempotent: removing an absent favorite is not an error.
	Remove(ctx context.Context, userID, anekdotID int64) error
}

// ExternalAnekdotSource is the driven port for an external anecdote provider.
type ExternalAnekdotSource interface {
	Random(ctx context.Context) (string, error)
}
