package usecase

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const (
	maxFeedbackMessageLength = 4000
	maxFeedbackContactLength = 200
	defaultFeedbackLimit     = 50
	maxFeedbackLimit         = 100
)

// FeedbackService serves the contact form use cases.
type FeedbackService struct {
	repository domain.FeedbackRepository
}

// NewFeedbackService wires the feedback use case with its port.
func NewFeedbackService(repository domain.FeedbackRepository) *FeedbackService {
	return &FeedbackService{repository: repository}
}

// Submit validates and stores a message for the administration.
func (s *FeedbackService) Submit(ctx context.Context, message, contact string) (domain.Feedback, error) {
	trimmedMessage := strings.TrimSpace(message)
	if err := validateFeedbackMessage(trimmedMessage); err != nil {
		return domain.Feedback{}, err
	}

	contactValue, hasContact, err := normalizeContact(contact)
	if err != nil {
		return domain.Feedback{}, err
	}

	var contactPointer *string
	if hasContact {
		contactPointer = &contactValue
	}

	feedback, err := s.repository.Create(ctx, trimmedMessage, contactPointer)
	if err != nil {
		return domain.Feedback{}, fmt.Errorf("creating feedback: %w", err)
	}

	return feedback, nil
}

// List returns feedback messages for the administrator.
func (s *FeedbackService) List(ctx context.Context, limit, offset int) ([]domain.Feedback, error) {
	if limit <= 0 || limit > maxFeedbackLimit {
		limit = defaultFeedbackLimit
	}
	if offset < 0 {
		offset = 0
	}

	feedback, err := s.repository.List(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listing feedback: %w", err)
	}

	return feedback, nil
}

func validateFeedbackMessage(message string) error {
	length := utf8.RuneCountInString(message)
	if length == 0 {
		return fmt.Errorf("%w: message must not be empty", domain.ErrInvalidInput)
	}
	if length > maxFeedbackMessageLength {
		return fmt.Errorf("%w: message must be at most %d characters", domain.ErrInvalidInput, maxFeedbackMessageLength)
	}

	return nil
}

func normalizeContact(contact string) (string, bool, error) {
	trimmed := strings.TrimSpace(contact)
	if trimmed == "" {
		return "", false, nil
	}
	if utf8.RuneCountInString(trimmed) > maxFeedbackContactLength {
		return "", false, fmt.Errorf(
			"%w: contact must be at most %d characters",
			domain.ErrInvalidInput,
			maxFeedbackContactLength,
		)
	}

	return trimmed, true, nil
}
