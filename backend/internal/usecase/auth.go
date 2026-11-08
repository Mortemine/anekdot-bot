package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const (
	minPasswordLength = 8
	// bcrypt silently ignores everything past 72 bytes, so longer passwords
	// are rejected instead of being truncated behind the user's back.
	maxPasswordLength    = 72
	maxEmailLength       = 254
	maxDisplayNameLength = 100
	sessionTokenBytes    = 32
	// passwordHashCost is the bcrypt work factor used for new passwords.
	passwordHashCost = 12
)

// AuthResult is a user together with the session issued for it.
type AuthResult struct {
	User      domain.User
	Token     string
	ExpiresAt time.Time
}

// AuthService registers users, signs them in and resolves session tokens.
type AuthService struct {
	users      domain.UserRepository
	sessions   domain.SessionRepository
	sessionTTL time.Duration
	logger     *slog.Logger
}

// NewAuthService wires the authentication use case with its ports.
func NewAuthService(
	users domain.UserRepository,
	sessions domain.SessionRepository,
	sessionTTL time.Duration,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		users:      users,
		sessions:   sessions,
		sessionTTL: sessionTTL,
		logger:     logger,
	}
}

// Register creates an account and immediately signs the user in.
func (s *AuthService) Register(ctx context.Context, email, password, displayName string) (AuthResult, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return AuthResult{}, err
	}

	if err := validatePassword(password); err != nil {
		return AuthResult{}, err
	}

	name, err := normalizeDisplayName(displayName, normalizedEmail)
	if err != nil {
		return AuthResult{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hashing password: %w", err)
	}

	user, err := s.users.Create(ctx, normalizedEmail, name, string(hash))
	if err != nil {
		return AuthResult{}, err
	}

	s.logger.InfoContext(ctx, "user registered", "user_id", user.ID)

	return s.startSession(ctx, user)
}

// Login verifies the credentials and issues a session.
//
// Every failure yields ErrUnauthorized so callers cannot enumerate accounts.
func (s *AuthService) Login(ctx context.Context, email, password string) (AuthResult, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return AuthResult{}, domain.ErrUnauthorized
	}

	if password == "" || len(password) > maxPasswordLength {
		return AuthResult{}, domain.ErrUnauthorized
	}

	credentials, err := s.users.ByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return AuthResult{}, domain.ErrUnauthorized
		}

		return AuthResult{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(credentials.PasswordHash), []byte(password)); err != nil {
		return AuthResult{}, domain.ErrUnauthorized
	}

	return s.startSession(ctx, credentials.User)
}

// Authenticate resolves a bearer token to its user.
func (s *AuthService) Authenticate(ctx context.Context, token string) (domain.User, error) {
	tokenHash, err := hashSessionToken(token)
	if err != nil {
		return domain.User{}, domain.ErrUnauthorized
	}

	user, err := s.sessions.ByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, domain.ErrUnauthorized
		}

		return domain.User{}, err
	}

	return user, nil
}

// Logout revokes a single session. Revoking an unknown token is not an error.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil
	}

	return s.sessions.Delete(ctx, hashToken(trimmed))
}

func (s *AuthService) startSession(ctx context.Context, user domain.User) (AuthResult, error) {
	token, tokenHash, err := newSessionToken()
	if err != nil {
		return AuthResult{}, err
	}

	expiresAt := time.Now().Add(s.sessionTTL)
	session := domain.Session{TokenHash: tokenHash, UserID: user.ID, ExpiresAt: expiresAt}

	if err := s.sessions.Create(ctx, session); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > maxEmailLength {
		return "", fmt.Errorf("%w: email must be 1 to %d characters", domain.ErrInvalidInput, maxEmailLength)
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email {
		return "", fmt.Errorf("%w: email is malformed", domain.ErrInvalidInput)
	}

	return email, nil
}

func validatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < minPasswordLength || length > maxPasswordLength {
		return fmt.Errorf(
			"%w: password must be %d to %d characters long",
			domain.ErrInvalidInput,
			minPasswordLength,
			maxPasswordLength,
		)
	}

	return nil
}

func normalizeDisplayName(raw, email string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		if local, _, found := strings.Cut(email, "@"); found {
			name = local
		}
	}

	if name == "" || utf8.RuneCountInString(name) > maxDisplayNameLength {
		return "", fmt.Errorf(
			"%w: display name must be 1 to %d characters long",
			domain.ErrInvalidInput,
			maxDisplayNameLength,
		)
	}

	return name, nil
}

func newSessionToken() (string, string, error) {
	buffer := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", "", fmt.Errorf("generating session token: %w", err)
	}

	token := hex.EncodeToString(buffer)

	return token, hashToken(token), nil
}

func hashSessionToken(token string) (string, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return "", fmt.Errorf("%w: session token is empty", domain.ErrInvalidInput)
	}

	return hashToken(trimmed), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}
