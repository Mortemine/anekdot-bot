package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexey/anekdotus/backend/internal/domain"
	"github.com/alexey/anekdotus/backend/internal/usecase"
)

type anekdotCollection struct {
	resource string
	list     func(ctx context.Context, userID int64) ([]domain.Anekdot, error)
	add      func(ctx context.Context, userID, anekdotID int64) error
	remove   func(ctx context.Context, userID, anekdotID int64) error
	logger   *slog.Logger
}

func NewLikeHandler(service *usecase.AnekdotService, logger *slog.Logger) *anekdotCollection {
	return &anekdotCollection{
		resource: "likes",
		list:     service.Likes,
		add:      service.AddLike,
		remove:   service.RemoveLike,
		logger:   logger,
	}
}

func NewFavoriteHandler(service *usecase.AnekdotService, logger *slog.Logger) *anekdotCollection {
	return &anekdotCollection{
		resource: "favorites",
		list:     service.Favorites,
		add:      service.AddFavorite,
		remove:   service.RemoveFavorite,
		logger:   logger,
	}
}

func (h *anekdotCollection) List(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	anekdots, err := h.list(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{Anekdots: toAnekdotsDTO(anekdots)})
}

func (h *anekdotCollection) Add(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	if err := h.add(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *anekdotCollection) Remove(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	if err := h.remove(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *anekdotCollection) anekdotID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	anekdotID, err := strconv.ParseInt(r.PathValue("anekdot_id"), 10, 64)
	if err != nil || anekdotID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Идентификатор анекдота должен быть положительным числом.")

		return 0, false
	}

	return anekdotID, true
}

func (h *anekdotCollection) handleError(w http.ResponseWriter, r *http.Request, err error) {
	writeRequestError(w, r, h.logger, h.resource+" request failed", err)
}
