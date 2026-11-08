package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":{"code":"internal_error","message":"internal server error"}}`, http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if _, err := w.Write(body); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Ничего не найдено.")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_input", "Проверьте введённые данные.")
	case errors.Is(err, domain.ErrUpstreamUnavailable):
		writeError(w, http.StatusBadGateway, "upstream_unavailable", "Внешний источник анекдотов сейчас недоступен.")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера.")
	}
}

func isExpectedError(err error) bool {
	return errors.Is(err, domain.ErrNotFound) ||
		errors.Is(err, domain.ErrInvalidInput) ||
		errors.Is(err, domain.ErrUpstreamUnavailable)
}
