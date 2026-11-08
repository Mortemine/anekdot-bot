package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

type feedbackRepositoryStub struct {
	created     domain.Feedback
	createErr   error
	messages    []domain.Feedback
	listErr     error
	lastMessage string
	lastContact *string
	lastLimit   int
	lastOffset  int
}

func (s *feedbackRepositoryStub) Create(
	_ context.Context,
	message string,
	contact *string,
) (domain.Feedback, error) {
	s.lastMessage = message
	s.lastContact = contact

	return s.created, s.createErr
}

func (s *feedbackRepositoryStub) List(_ context.Context, limit, offset int) ([]domain.Feedback, error) {
	s.lastLimit = limit
	s.lastOffset = offset

	return s.messages, s.listErr
}

func TestFeedbackServiceSubmit(t *testing.T) {
	t.Run("trims message and clears empty contact", func(t *testing.T) {
		repository := &feedbackRepositoryStub{created: domain.Feedback{ID: 1, Message: "привет"}}
		service := NewFeedbackService(repository)

		if _, err := service.Submit(context.Background(), "  привет  ", "   "); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
		if repository.lastMessage != "привет" {
			t.Errorf("stored message = %q, want %q", repository.lastMessage, "привет")
		}
		if repository.lastContact != nil {
			t.Errorf("stored contact = %q, want nil", *repository.lastContact)
		}
	})

	t.Run("keeps contact", func(t *testing.T) {
		repository := &feedbackRepositoryStub{}
		service := NewFeedbackService(repository)

		if _, err := service.Submit(context.Background(), "привет", " @user "); err != nil {
			t.Fatalf("Submit() error = %v", err)
		}
		if repository.lastContact == nil || *repository.lastContact != "@user" {
			t.Errorf("stored contact = %v, want @user", repository.lastContact)
		}
	})

	tests := []struct {
		name    string
		message string
		contact string
		wantErr bool
	}{
		{name: "empty message", message: "   ", wantErr: true},
		{name: "message too long", message: strings.Repeat("а", 4001), wantErr: true},
		{name: "contact too long", message: "привет", contact: strings.Repeat("a", 201), wantErr: true},
		{name: "message boundary", message: strings.Repeat("а", 4000)},
		{name: "contact boundary", message: "привет", contact: strings.Repeat("a", 200)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewFeedbackService(&feedbackRepositoryStub{})

			_, err := service.Submit(context.Background(), test.message, test.contact)
			if test.wantErr {
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Fatalf("Submit() error = %v, want ErrInvalidInput", err)
				}

				return
			}
			if err != nil {
				t.Fatalf("Submit() error = %v, want nil", err)
			}
		})
	}

	t.Run("repository failure", func(t *testing.T) {
		repository := &feedbackRepositoryStub{createErr: errors.New("boom")}
		service := NewFeedbackService(repository)

		if _, err := service.Submit(context.Background(), "привет", ""); err == nil {
			t.Fatal("Submit() error = nil, want error")
		}
	})
}

func TestFeedbackServiceList(t *testing.T) {
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantLimit  int
		wantOffset int
	}{
		{name: "defaults", limit: 0, offset: 0, wantLimit: 50, wantOffset: 0},
		{name: "limit too large", limit: 500, offset: 0, wantLimit: 50, wantOffset: 0},
		{name: "negative offset", limit: 10, offset: -5, wantLimit: 10, wantOffset: 0},
		{name: "valid", limit: 25, offset: 50, wantLimit: 25, wantOffset: 50},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &feedbackRepositoryStub{}
			service := NewFeedbackService(repository)

			if _, err := service.List(context.Background(), test.limit, test.offset); err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if repository.lastLimit != test.wantLimit {
				t.Errorf("limit = %d, want %d", repository.lastLimit, test.wantLimit)
			}
			if repository.lastOffset != test.wantOffset {
				t.Errorf("offset = %d, want %d", repository.lastOffset, test.wantOffset)
			}
		})
	}

	t.Run("repository failure", func(t *testing.T) {
		service := NewFeedbackService(&feedbackRepositoryStub{listErr: errors.New("boom")})

		if _, err := service.List(context.Background(), 10, 0); err == nil {
			t.Fatal("List() error = nil, want error")
		}
	})
}
