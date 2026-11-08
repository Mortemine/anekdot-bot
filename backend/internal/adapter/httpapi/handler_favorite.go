package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

// FavoriteHandler exposes the saved anecdote endpoints.
type FavoriteHandler struct {
	service *usecase.AnekdotService
	logger  *slog.Logger
}

// NewFavoriteHandler builds the favorites HTTP handler.
func NewFavoriteHandler(service *usecase.AnekdotService, logger *slog.Logger) *FavoriteHandler {
	return &FavoriteHandler{service: service, logger: logger}
}

// List handles GET /api/v1/favorites.
func (h *FavoriteHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	favorites, err := h.service.Favorites(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{Anekdots: toAnekdotsDTO(favorites)})
}

// Add handles PUT /api/v1/favorites/{anekdot_id}.
func (h *FavoriteHandler) Add(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	if err := h.service.AddFavorite(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Remove handles DELETE /api/v1/favorites/{anekdot_id}.
func (h *FavoriteHandler) Remove(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	if err := h.service.RemoveFavorite(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FavoriteHandler) anekdotID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	anekdotID, err := strconv.ParseInt(r.PathValue("anekdot_id"), 10, 64)
	if err != nil || anekdotID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Идентификатор анекдота должен быть положительным числом.")

		return 0, false
	}

	return anekdotID, true
}

func (h *FavoriteHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if !isExpectedError(err) {
		h.logger.ErrorContext(
			r.Context(),
			"favorites request failed",
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
