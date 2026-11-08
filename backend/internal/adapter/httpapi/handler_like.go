package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

type LikeHandler struct {
	service *usecase.AnekdotService
	logger  *slog.Logger
}

func NewLikeHandler(service *usecase.AnekdotService, logger *slog.Logger) *LikeHandler {
	return &LikeHandler{service: service, logger: logger}
}

func (h *LikeHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	likes, err := h.service.Likes(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{Anekdots: toAnekdotsDTO(likes)})
}

func (h *LikeHandler) Add(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	if err := h.service.AddLike(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *LikeHandler) Remove(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := h.anekdotID(w, r)
	if !ok {
		return
	}

	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	if err := h.service.RemoveLike(r.Context(), user.ID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *LikeHandler) anekdotID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	anekdotID, err := strconv.ParseInt(r.PathValue("anekdot_id"), 10, 64)
	if err != nil || anekdotID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "Идентификатор анекдота должен быть положительным числом.")

		return 0, false
	}

	return anekdotID, true
}

func (h *LikeHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if !isExpectedError(err) {
		h.logger.ErrorContext(
			r.Context(),
			"likes request failed",
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
