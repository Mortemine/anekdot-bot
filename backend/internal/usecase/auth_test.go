package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

type userRepositoryStub struct {
	byEmail   map[string]domain.Credentials
	byID      map[int64]domain.User
	nextID    int64
	createErr error
	lookupErr error
}

func newUserRepositoryStub() *userRepositoryStub {
	return &userRepositoryStub{
		byEmail: map[string]domain.Credentials{},
		byID:    map[int64]domain.User{},
	}
}

func (s *userRepositoryStub) Create(_ context.Context, email, displayName, passwordHash string) (domain.User, error) {
	if s.createErr != nil {
		return domain.User{}, s.createErr
	}

	if _, ok := s.byEmail[email]; ok {
		return domain.User{}, domain.ErrConflict
	}

	s.nextID++
	user := domain.User{ID: s.nextID, Email: email, DisplayName: displayName, CreatedAt: time.Unix(0, 0).UTC()}
	s.byID[user.ID] = user
	s.byEmail[email] = domain.Credentials{User: user, PasswordHash: passwordHash}

	return user, nil
}

func (s *userRepositoryStub) ByEmail(_ context.Context, email string) (domain.Credentials, error) {
	if s.lookupErr != nil {
		return domain.Credentials{}, s.lookupErr
	}

	credentials, ok := s.byEmail[email]
	if !ok {
		return domain.Credentials{}, domain.ErrNotFound
	}

	return credentials, nil
}

func (s *userRepositoryStub) ByID(_ context.Context, id int64) (domain.User, error) {
	user, ok := s.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return user, nil
}

type sessionRepositoryStub struct {
	users     *userRepositoryStub
	sessions  map[string]domain.Session
	createErr error
	lookupErr error
	deleteErr error
	deleted   []string
}

func newSessionRepositoryStub(users *userRepositoryStub) *sessionRepositoryStub {
	return &sessionRepositoryStub{users: users, sessions: map[string]domain.Session{}}
}

func (s *sessionRepositoryStub) Create(_ context.Context, session domain.Session) error {
	if s.createErr != nil {
		return s.createErr
	}

	s.sessions[session.TokenHash] = session

	return nil
}

func (s *sessionRepositoryStub) ByTokenHash(_ context.Context, tokenHash string) (domain.User, error) {
	if s.lookupErr != nil {
		return domain.User{}, s.lookupErr
	}

	session, ok := s.sessions[tokenHash]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	if time.Now().After(session.ExpiresAt) {
		return domain.User{}, domain.ErrNotFound
	}

	return s.users.byID[session.UserID], nil
}

func (s *sessionRepositoryStub) Delete(_ context.Context, tokenHash string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	delete(s.sessions, tokenHash)
	s.deleted = append(s.deleted, tokenHash)

	return nil
}

func newAuthFixture() (*AuthService, *userRepositoryStub, *sessionRepositoryStub) {
	users := newUserRepositoryStub()
	sessions := newSessionRepositoryStub(users)

	return NewAuthService(users, sessions, time.Hour, discardLogger()), users, sessions
}

func TestAuthServiceRegister(t *testing.T) {
	service, users, sessions := newAuthFixture()

	result, err := service.Register(context.Background(), " Reader@Example.com ", "password123", "  ")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if result.User.Email != "reader@example.com" {
		t.Errorf("email = %q, want normalized", result.User.Email)
	}
	if result.User.DisplayName != "reader" {
		t.Errorf("display name = %q, want reader", result.User.DisplayName)
	}
	if result.Token == "" {
		t.Error("token is empty")
	}
	if len(sessions.sessions) != 1 {
		t.Errorf("sessions = %d, want 1", len(sessions.sessions))
	}
	if !result.ExpiresAt.After(time.Now()) {
		t.Errorf("expires_at = %v, want in the future", result.ExpiresAt)
	}

	credentials := users.byEmail["reader@example.com"]
	if credentials.PasswordHash == "password123" {
		t.Error("password is stored in plain text")
	}
	if !strings.HasPrefix(credentials.PasswordHash, "$2") {
		t.Errorf("password hash = %q, want bcrypt", credentials.PasswordHash)
	}
}

func TestAuthServiceRegisterRejects(t *testing.T) {
	tests := []struct {
		name        string
		email       string
		password    string
		displayName string
	}{
		{name: "empty email", email: "", password: "password123"},
		{name: "malformed email", email: "not-an-email", password: "password123"},
		{name: "short password", email: "reader@example.com", password: "short"},
		{name: "long password", email: "reader@example.com", password: strings.Repeat("a", maxPasswordLength+1)},
		{
			name:        "long display name",
			email:       "reader@example.com",
			password:    "password123",
			displayName: strings.Repeat("я", maxDisplayNameLength+1),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _, _ := newAuthFixture()

			if _, err := service.Register(context.Background(), test.email, test.password, test.displayName); !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("Register() error = %v, want ErrInvalidInput", err)
			}
		})
	}

	t.Run("duplicate email", func(t *testing.T) {
		service, _, _ := newAuthFixture()

		if _, err := service.Register(context.Background(), "reader@example.com", "password123", ""); err != nil {
			t.Fatalf("first Register() error = %v", err)
		}

		if _, err := service.Register(context.Background(), "reader@example.com", "password123", ""); !errors.Is(err, domain.ErrConflict) {
			t.Fatalf("second Register() error = %v, want ErrConflict", err)
		}
	})
}

func TestAuthServiceLogin(t *testing.T) {
	service, _, _ := newAuthFixture()

	if _, err := service.Register(context.Background(), "reader@example.com", "password123", "Читатель"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	t.Run("success", func(t *testing.T) {
		result, err := service.Login(context.Background(), " READER@example.com ", "password123")
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		if result.Token == "" {
			t.Error("token is empty")
		}
		if result.User.DisplayName != "Читатель" {
			t.Errorf("display name = %q", result.User.DisplayName)
		}
	})

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{name: "wrong password", email: "reader@example.com", password: "wrong-password"},
		{name: "unknown email", email: "nobody@example.com", password: "password123"},
		{name: "empty password", email: "reader@example.com", password: ""},
		{name: "malformed email", email: "not-an-email", password: "password123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := service.Login(context.Background(), test.email, test.password); !errors.Is(err, domain.ErrUnauthorized) {
				t.Fatalf("Login() error = %v, want ErrUnauthorized", err)
			}
		})
	}
}

func TestAuthServiceAuthenticate(t *testing.T) {
	service, _, sessions := newAuthFixture()

	registered, err := service.Register(context.Background(), "reader@example.com", "password123", "")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		user, authErr := service.Authenticate(context.Background(), registered.Token)
		if authErr != nil {
			t.Fatalf("Authenticate() error = %v", authErr)
		}
		if user.Email != "reader@example.com" {
			t.Errorf("email = %q", user.Email)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		if _, err := service.Authenticate(context.Background(), "   "); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("Authenticate() error = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		if _, err := service.Authenticate(context.Background(), "unknown"); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("Authenticate() error = %v, want ErrUnauthorized", err)
		}
	})

	t.Run("expired session", func(t *testing.T) {
		sessions.sessions[hashToken(registered.Token)] = domain.Session{
			TokenHash: hashToken(registered.Token),
			UserID:    registered.User.ID,
			ExpiresAt: time.Now().Add(-time.Minute),
		}

		if _, err := service.Authenticate(context.Background(), registered.Token); !errors.Is(err, domain.ErrUnauthorized) {
			t.Fatalf("Authenticate() error = %v, want ErrUnauthorized", err)
		}
	})
}

func TestAuthServiceLogout(t *testing.T) {
	service, _, sessions := newAuthFixture()

	registered, err := service.Register(context.Background(), "reader@example.com", "password123", "")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := service.Logout(context.Background(), registered.Token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if len(sessions.deleted) != 1 {
		t.Errorf("deleted = %v, want one token", sessions.deleted)
	}
	if _, err := service.Authenticate(context.Background(), registered.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("Authenticate() after logout error = %v, want ErrUnauthorized", err)
	}

	t.Run("unknown token is not an error", func(t *testing.T) {
		if err := service.Logout(context.Background(), "   "); err != nil {
			t.Fatalf("Logout() error = %v", err)
		}
	})
}
