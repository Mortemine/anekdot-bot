// Package domain contains the core entities, ports and errors of the service.
package domain

import "time"

// Category is a named group of anecdotes.
type Category struct {
	ID   int
	Name string
}

// CategorySummary is a category enriched with the number of anecdotes it holds.
type CategorySummary struct {
	Category
	AnekdotCount int
}

// Anekdot is a single joke.
//
// AuthorID is nil for anecdotes that come from the imported archive; it points
// to the registered user who submitted the anecdote otherwise.
type Anekdot struct {
	ID           int64
	CategoryID   int
	CategoryName string
	Text         string
	AuthorID     *int64
	CreatedAt    time.Time
}

// User is a registered reader who can keep favorites and publish anecdotes.
type User struct {
	ID          int64
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

// Credentials is a user together with the stored password hash.
//
// It exists so the hash never travels through the layers that only need to
// identify a user.
type Credentials struct {
	User
	PasswordHash string
}

// Session is a bearer token issued after a successful login.
//
// Only the hash of the token is ever stored, so a database leak does not hand
// out usable credentials.
type Session struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
}
