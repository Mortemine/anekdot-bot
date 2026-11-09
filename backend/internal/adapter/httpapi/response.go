package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const internalErrorMessage = "Внутренняя ошибка сервера."

type errorMapping struct {
	err     error
	status  int
	code    string
	message string
}

var errorMappings = []errorMapping{
	{domain.ErrNotFound, http.StatusNotFound, "not_found", "Ничего не найдено."},
	{domain.ErrInvalidInput, http.StatusBadRequest, "invalid_input", "Проверьте введённые данные."},
	{domain.ErrUnauthorized, http.StatusUnauthorized, "unauthorized", "Неверная почта или пароль."},
	{domain.ErrConflict, http.StatusConflict, "conflict", "Такая запись уже есть."},
	{domain.ErrForbidden, http.StatusForbidden, "forbidden", "Недостаточно прав для этого действия."},
	{
		domain.ErrUpstreamUnavailable,
		http.StatusBadGateway,
		"upstream_unavailable",
		"Внешний источник анекдотов сейчас недоступен.",
	},
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"` + internalErrorMessage + `"}}`))

		return
	}

	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func lookupErrorMapping(err error) (errorMapping, bool) {
	for _, mapping := range errorMappings {
		if errors.Is(err, mapping.err) {
			return mapping, true
		}
	}

	return errorMapping{}, false
}

func writeDomainError(w http.ResponseWriter, err error) {
	mapping, ok := lookupErrorMapping(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", internalErrorMessage)

		return
	}

	writeError(w, mapping.status, mapping.code, mapping.message)
}

func writeRequestError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, message string, err error) {
	if _, ok := lookupErrorMapping(err); !ok {
		logger.ErrorContext(
			r.Context(),
			message,
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
