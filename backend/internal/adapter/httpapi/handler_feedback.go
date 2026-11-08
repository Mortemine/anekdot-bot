package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

const (
	maxFeedbackBodyBytes = 64 << 10

	defaultListLimit = 50
	minListLimit     = 1
	maxListLimit     = 100
	maxListOffset    = 1_000_000
)

type createFeedbackRequest struct {
	Message string `json:"message"`
	Contact string `json:"contact"`
}

type feedbackDTO struct {
	ID        int64     `json:"id"`
	Message   string    `json:"message"`
	Contact   *string   `json:"contact"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type feedbackResponse struct {
	Feedback feedbackDTO `json:"feedback"`
}

type feedbackListResponse struct {
	Feedback   []feedbackDTO `json:"feedback"`
	Pagination paginationDTO `json:"pagination"`
}

type paginationDTO struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// FeedbackHandler exposes the contact form endpoints.
type FeedbackHandler struct {
	service *usecase.FeedbackService
	logger  *slog.Logger
}

// NewFeedbackHandler builds the feedback HTTP handler.
func NewFeedbackHandler(service *usecase.FeedbackService, logger *slog.Logger) *FeedbackHandler {
	return &FeedbackHandler{service: service, logger: logger}
}

// Create handles POST /api/v1/feedback.
func (h *FeedbackHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxFeedbackBodyBytes)

	var request createFeedbackRequest
	if err := decodeJSON(r, &request); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Сообщение слишком большое.")

			return
		}

		writeError(w, http.StatusBadRequest, "invalid_json", "Не удалось прочитать JSON.")

		return
	}

	feedback, err := h.service.Submit(r.Context(), request.Message, request.Contact)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusCreated, feedbackResponse{Feedback: toFeedbackDTO(feedback)})
}

// List handles GET /api/v1/admin/feedback.
func (h *FeedbackHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := parseBoundedInt(r.URL.Query().Get("limit"), defaultListLimit, minListLimit, maxListLimit)
	offset := parseBoundedInt(r.URL.Query().Get("offset"), 0, 0, maxListOffset)

	feedback, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	messages := make([]feedbackDTO, 0, len(feedback))
	for _, message := range feedback {
		messages = append(messages, toFeedbackDTO(message))
	}

	writeJSON(w, http.StatusOK, feedbackListResponse{
		Feedback:   messages,
		Pagination: paginationDTO{Limit: limit, Offset: offset},
	})
}

func (h *FeedbackHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if !isExpectedError(err) {
		h.logger.ErrorContext(
			r.Context(),
			"feedback request failed",
			"error", err,
			"path", r.URL.Path,
			"request_id", requestIDFromContext(r.Context()),
		)
	}

	writeDomainError(w, err)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}

	return nil
}

func parseBoundedInt(raw string, fallback, minimum, maximum int) int {
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}

	return value
}

func toFeedbackDTO(feedback domain.Feedback) feedbackDTO {
	return feedbackDTO{
		ID:        feedback.ID,
		Message:   feedback.Message,
		Contact:   feedback.Contact,
		Status:    string(feedback.Status),
		CreatedAt: feedback.CreatedAt,
	}
}
