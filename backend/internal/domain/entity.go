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
type Anekdot struct {
	ID           int64
	CategoryID   int
	CategoryName string
	Text         string
	CreatedAt    time.Time
}

// FeedbackStatus describes the moderation state of a feedback message.
type FeedbackStatus string

const (
	// FeedbackStatusNew marks a message nobody has reviewed yet.
	FeedbackStatusNew FeedbackStatus = "new"
	// FeedbackStatusRead marks a reviewed message.
	FeedbackStatusRead FeedbackStatus = "read"
	// FeedbackStatusArchived marks a message moved out of the inbox.
	FeedbackStatusArchived FeedbackStatus = "archived"
)

// Feedback is a message sent to the administration through the contact form.
type Feedback struct {
	ID        int64
	Message   string
	Contact   *string
	Status    FeedbackStatus
	CreatedAt time.Time
}
