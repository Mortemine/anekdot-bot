package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
)

func decodeRequest(w http.ResponseWriter, r *http.Request, limit int64, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(
				w,
				http.StatusRequestEntityTooLarge,
				"payload_too_large",
				"Тело запроса слишком большое.",
			)

			return false
		}

		writeError(w, http.StatusBadRequest, "invalid_json", "Не удалось разобрать тело запроса.")

		return false
	}

	if decoder.More() {
		writeError(w, http.StatusBadRequest, "invalid_json", "Ожидался один JSON-объект.")

		return false
	}

	return true
}
