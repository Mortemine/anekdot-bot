package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

const maxAuthBodyBytes = 8 << 10

type userDTO struct {
	ID          int64     `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type authResponse struct {
	User      userDTO   `json:"user"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type sessionResponse struct {
	User userDTO `json:"user"`
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthHandler exposes registration, login and session endpoints.
type AuthHandler struct {
	service *usecase.AuthService
	logger  *slog.Logger
}

// NewAuthHandler builds the authentication HTTP handler.
func NewAuthHandler(service *usecase.AuthService, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{service: service, logger: logger}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var payload registerRequest
	if !decodeRequest(w, r, maxAuthBodyBytes, &payload) {
		return
	}

	result, err := h.service.Register(r.Context(), payload.Email, payload.Password, payload.DisplayName)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusCreated, toAuthResponse(result))
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var payload loginRequest
	if !decodeRequest(w, r, maxAuthBodyBytes, &payload) {
		return
	}

	result, err := h.service.Login(r.Context(), payload.Email, payload.Password)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, toAuthResponse(result))
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.Logout(r.Context(), bearerToken(r)); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /api/v1/auth/me.
//
// The authentication middleware has already resolved the user, so a missing
// user here means the route was wired without it.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	writeJSON(w, http.StatusOK, sessionResponse{User: toUserDTO(user)})
}

func (h *AuthHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if !isExpectedError(err) {
		h.logger.ErrorContext(
			r.Context(),
			"authentication request failed",
			"error", err,
			"path", r.URL.Path,
			"request_id", requestIDFromContext(r.Context()),
		)
	}

	if errors.Is(err, domain.ErrUnauthorized) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="anekdot"`)
	}

	writeDomainError(w, err)
}

func toAuthResponse(result usecase.AuthResult) authResponse {
	return authResponse{
		User:      toUserDTO(result.User),
		Token:     result.Token,
		ExpiresAt: result.ExpiresAt,
	}
}

func toUserDTO(user domain.User) userDTO {
	return userDTO{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}
}
