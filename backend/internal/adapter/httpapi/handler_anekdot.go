package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

const (
	externalAnekdotSource = "baneks.site"
	maxAnekdotBodyBytes   = 16 << 10
)

type categoryDTO struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	AnekdotCount int    `json:"anekdot_count"`
}

type anekdotDTO struct {
	ID           int64  `json:"id"`
	CategoryID   int    `json:"category_id"`
	CategoryName string `json:"category_name"`
	Text         string `json:"text"`
	AuthorID     *int64 `json:"author_id,omitempty"`
}

type externalAnekdotDTO struct {
	Text   string `json:"text"`
	Source string `json:"source"`
}

type categoriesResponse struct {
	Categories []categoryDTO `json:"categories"`
}

type anekdotResponse struct {
	Anekdot anekdotDTO `json:"anekdot"`
}

type anekdotsResponse struct {
	Anekdots []anekdotDTO `json:"anekdots"`
}

type externalAnekdotResponse struct {
	Anekdot externalAnekdotDTO `json:"anekdot"`
}

type createAnekdotRequest struct {
	CategoryID int    `json:"category_id"`
	Text       string `json:"text"`
}

type AnekdotHandler struct {
	service *usecase.AnekdotService
	logger  *slog.Logger
}

func NewAnekdotHandler(service *usecase.AnekdotService, logger *slog.Logger) *AnekdotHandler {
	return &AnekdotHandler{service: service, logger: logger}
}

func (h *AnekdotHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	summaries, err := h.service.ListCategories(r.Context())
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	categories := make([]categoryDTO, 0, len(summaries))
	for _, summary := range summaries {
		categories = append(categories, categoryDTO{
			ID:           summary.ID,
			Name:         summary.Name,
			AnekdotCount: summary.AnekdotCount,
		})
	}

	writeJSON(w, http.StatusOK, categoriesResponse{Categories: categories})
}

func (h *AnekdotHandler) Random(w http.ResponseWriter, r *http.Request) {
	categoryID, filtered, err := parseCategoryID(r.URL.Query().Get("category_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Параметр category_id должен быть неотрицательным числом.")

		return
	}

	var filter *int
	if filtered {
		filter = &categoryID
	}

	anekdot, err := h.service.Random(r.Context(), filter)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotResponse{Anekdot: toAnekdotDTO(anekdot)})
}

func (h *AnekdotHandler) External(w http.ResponseWriter, r *http.Request) {
	text, err := h.service.External(r.Context())
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, externalAnekdotResponse{
		Anekdot: externalAnekdotDTO{Text: text, Source: externalAnekdotSource},
	})
}

func (h *AnekdotHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	var payload createAnekdotRequest
	if !decodeRequest(w, r, maxAnekdotBodyBytes, &payload) {
		return
	}

	created, err := h.service.Create(r.Context(), user.ID, payload.CategoryID, payload.Text)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusCreated, anekdotResponse{Anekdot: toAnekdotDTO(created)})
}

func (h *AnekdotHandler) Mine(w http.ResponseWriter, r *http.Request) {
	user, ok := UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

		return
	}

	anekdots, err := h.service.Mine(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{Anekdots: toAnekdotsDTO(anekdots)})
}

func toAnekdotsDTO(anekdots []domain.Anekdot) []anekdotDTO {
	payload := make([]anekdotDTO, 0, len(anekdots))
	for _, anekdot := range anekdots {
		payload = append(payload, toAnekdotDTO(anekdot))
	}

	return payload
}

func (h *AnekdotHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if !isExpectedError(err) {
		h.logger.ErrorContext(
			r.Context(),
			"anecdote request failed",
			"error", err,
			"path", r.URL.Path,
			"request_id", requestIDFromContext(r.Context()),
		)
	}

	writeDomainError(w, err)
}

func parseCategoryID(raw string) (int, bool, error) {
	if raw == "" {
		return 0, false, nil
	}

	categoryID, err := strconv.Atoi(raw)
	if err != nil || categoryID < 0 {
		return 0, false, fmt.Errorf("%w: category_id must not be negative", domain.ErrInvalidInput)
	}

	return categoryID, true, nil
}

func toAnekdotDTO(anekdot domain.Anekdot) anekdotDTO {
	return anekdotDTO{
		ID:           anekdot.ID,
		CategoryID:   anekdot.CategoryID,
		CategoryName: anekdot.CategoryName,
		Text:         anekdot.Text,
		AuthorID:     anekdot.AuthorID,
	}
}
