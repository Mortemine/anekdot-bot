package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

// FeedbackRepository stores feedback messages in PostgreSQL.
type FeedbackRepository struct {
	pool *pgxpool.Pool
}

// NewFeedbackRepository returns a PostgreSQL backed feedback repository.
func NewFeedbackRepository(pool *pgxpool.Pool) *FeedbackRepository {
	return &FeedbackRepository{pool: pool}
}

// Create inserts a feedback message and returns the stored row.
func (r *FeedbackRepository) Create(ctx context.Context, message string, contact *string) (domain.Feedback, error) {
	var contactValue any
	if contact != nil {
		contactValue = *contact
	}

	var (
		feedback      domain.Feedback
		storedContact pgtype.Text
		status        string
	)

	err := r.pool.QueryRow(
		ctx,
		`INSERT INTO feedback_messages (message, contact)
		 VALUES ($1, $2)
		 RETURNING id, message, contact, status, created_at`,
		message,
		contactValue,
	).Scan(&feedback.ID, &feedback.Message, &storedContact, &status, &feedback.CreatedAt)
	if err != nil {
		return domain.Feedback{}, fmt.Errorf("inserting feedback: %w", err)
	}

	feedback.Contact = textPointer(storedContact)
	feedback.Status = domain.FeedbackStatus(status)

	return feedback, nil
}

// List returns feedback messages newest first.
func (r *FeedbackRepository) List(ctx context.Context, limit, offset int) ([]domain.Feedback, error) {
	rows, err := r.pool.Query(
		ctx,
		`SELECT id, message, contact, status, created_at
		 FROM feedback_messages
		 ORDER BY created_at DESC, id DESC
		 LIMIT $1 OFFSET $2`,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("querying feedback: %w", err)
	}
	defer rows.Close()

	messages := []domain.Feedback{}
	for rows.Next() {
		var (
			feedback domain.Feedback
			contact  pgtype.Text
			status   string
		)
		if err := rows.Scan(&feedback.ID, &feedback.Message, &contact, &status, &feedback.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning feedback: %w", err)
		}

		feedback.Contact = textPointer(contact)
		feedback.Status = domain.FeedbackStatus(status)
		messages = append(messages, feedback)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating feedback: %w", err)
	}

	return messages, nil
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}

	text := value.String

	return &text
}
