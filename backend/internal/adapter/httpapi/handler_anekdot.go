package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/alexey/anekdotus/backend/internal/domain"
	"github.com/alexey/anekdotus/backend/internal/usecase"
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
	ID            int64  `json:"id"`
	CategoryID    int    `json:"category_id"`
	CategoryName  string `json:"category_name"`
	Text          string `json:"text"`
	LikeCount     int    `json:"like_count"`
	FavoriteCount int    `json:"favorite_count"`
	AuthorID      *int64 `json:"author_id,omitempty"`
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
	Total    int          `json:"total"`
	Limit    int          `json:"limit"`
	Offset   int          `json:"offset"`
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
	filter, ok := parseCategoryFilter(w, r)
	if !ok {
		return
	}

	anekdot, err := h.service.Random(r.Context(), filter)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotResponse{Anekdot: toAnekdotDTO(anekdot)})
}

func (h *AnekdotHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseCategoryFilter(w, r)
	if !ok {
		return
	}

	query := r.URL.Query()

	limit, ok := parsePageParam(w, query.Get("limit"))
	if !ok {
		return
	}

	offset, ok := parsePageParam(w, query.Get("offset"))
	if !ok {
		return
	}

	sort, ok := parseAnekdotSort(w, query.Get("sort"))
	if !ok {
		return
	}

	page, err := h.service.Anekdots(r.Context(), filter, limit, offset, sort)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{
		Anekdots: toAnekdotsDTO(page.Anekdots),
		Total:    page.Total,
		Limit:    page.Limit,
		Offset:   page.Offset,
	})
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
	user, ok := requireUser(w, r)
	if !ok {
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
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	anekdots, err := h.service.Mine(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, anekdotsResponse{
		Anekdots: toAnekdotsDTO(anekdots),
		Total:    len(anekdots),
		Limit:    len(anekdots),
	})
}

func toAnekdotsDTO(anekdots []domain.Anekdot) []anekdotDTO {
	payload := make([]anekdotDTO, 0, len(anekdots))
	for _, anekdot := range anekdots {
		payload = append(payload, toAnekdotDTO(anekdot))
	}

	return payload
}

func (h *AnekdotHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	writeRequestError(w, r, h.logger, "anecdote request failed", err)
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

func parseCategoryFilter(w http.ResponseWriter, r *http.Request) (*int, bool) {
	categoryID, filtered, err := parseCategoryID(r.URL.Query().Get("category_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "Параметр category_id должен быть неотрицательным числом.")

		return nil, false
	}

	if !filtered {
		return nil, true
	}

	return &categoryID, true
}

func parsePageParam(w http.ResponseWriter, raw string) (int, bool) {
	if raw == "" {
		return 0, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_input",
			"Параметры limit и offset должны быть неотрицательными числами.",
		)

		return 0, false
	}

	return value, true
}

func parseAnekdotSort(w http.ResponseWriter, raw string) (domain.AnekdotSort, bool) {
	switch domain.AnekdotSort(raw) {
	case "":
		return domain.AnekdotSortCreated, true
	case domain.AnekdotSortCreated, domain.AnekdotSortLikes, domain.AnekdotSortFavorites:
		return domain.AnekdotSort(raw), true
	default:
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_input",
			"Параметр sort должен быть одним из: created, likes, favorites.",
		)

		return "", false
	}
}

func toAnekdotDTO(anekdot domain.Anekdot) anekdotDTO {
	return anekdotDTO{
		ID:            anekdot.ID,
		CategoryID:    anekdot.CategoryID,
		CategoryName:  anekdot.CategoryName,
		Text:          anekdot.Text,
		LikeCount:     anekdot.LikeCount,
		FavoriteCount: anekdot.FavoriteCount,
		AuthorID:      anekdot.AuthorID,
	}
}
