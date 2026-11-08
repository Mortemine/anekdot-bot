package domain

import "context"

type CategoryRepository interface {
	List(ctx context.Context) ([]Category, error)
	Get(ctx context.Context, id int) (Category, error)
}

type AnekdotRepository interface {
	Random(ctx context.Context) (Anekdot, error)
	RandomByCategory(ctx context.Context, categoryID int) (Anekdot, error)
	CountsByCategory(ctx context.Context) (map[int]int, error)
	ByID(ctx context.Context, id int64) (Anekdot, error)
	Create(ctx context.Context, anekdot Anekdot) (Anekdot, error)
	ListByAuthor(ctx context.Context, authorID int64, limit int) ([]Anekdot, error)
	List(ctx context.Context, categoryID *int, limit, offset int) ([]Anekdot, error)
}

type UserRepository interface {
	Create(ctx context.Context, email, displayName, passwordHash string) (User, error)
	ByEmail(ctx context.Context, email string) (Credentials, error)
	ByID(ctx context.Context, id int64) (User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session Session) error
	ByTokenHash(ctx context.Context, tokenHash string) (User, error)
	Delete(ctx context.Context, tokenHash string) error
}

type LikeRepository interface {
	List(ctx context.Context, userID int64) ([]Anekdot, error)
	Add(ctx context.Context, userID, anekdotID int64) error
	Remove(ctx context.Context, userID, anekdotID int64) error
}

type FavoriteRepository interface {
	List(ctx context.Context, userID int64) ([]Anekdot, error)
	Add(ctx context.Context, userID, anekdotID int64) error
	Remove(ctx context.Context, userID, anekdotID int64) error
}

type ExternalAnekdotSource interface {
	Random(ctx context.Context) (string, error)
}
