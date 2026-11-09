package httpapi

import (
	"context"
	"log/slog"
	"net/http"

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
	return newAnekdotCollection("likes", service.Likes, service.AddLike, service.RemoveLike, logger)
}

func NewFavoriteHandler(service *usecase.AnekdotService, logger *slog.Logger) *anekdotCollection {
	return newAnekdotCollection("favorites", service.Favorites, service.AddFavorite, service.RemoveFavorite, logger)
}

func newAnekdotCollection(
	resource string,
	list func(ctx context.Context, userID int64) ([]domain.Anekdot, error),
	add func(ctx context.Context, userID, anekdotID int64) error,
	remove func(ctx context.Context, userID, anekdotID int64) error,
	logger *slog.Logger,
) *anekdotCollection {
	return &anekdotCollection{
		resource: resource,
		list:     list,
		add:      add,
		remove:   remove,
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
	anekdotID, ok := parsePathID(w, r, "anekdot_id", "анекдота")
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
	anekdotID, ok := parsePathID(w, r, "anekdot_id", "анекдота")
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

func (h *anekdotCollection) handleError(w http.ResponseWriter, r *http.Request, err error) {
	writeRequestError(w, r, h.logger, h.resource+" request failed", err)
}
