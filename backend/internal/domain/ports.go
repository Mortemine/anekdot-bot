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
}

// FeedbackRepository is the driven port for feedback storage.
type FeedbackRepository interface {
	Create(ctx context.Context, message string, contact *string) (Feedback, error)
	List(ctx context.Context, limit, offset int) ([]Feedback, error)
}

// ExternalAnekdotSource is the driven port for an external anecdote provider.
type ExternalAnekdotSource interface {
	Random(ctx context.Context) (string, error)
}
