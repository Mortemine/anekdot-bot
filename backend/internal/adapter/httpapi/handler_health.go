package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

type databasePinger interface {
	Ping(ctx context.Context) error
}

type healthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// HealthHandler reports service and database health.
type HealthHandler struct {
	pinger databasePinger
	logger *slog.Logger
}

// NewHealthHandler builds the health endpoint handler.
func NewHealthHandler(pinger databasePinger, logger *slog.Logger) *HealthHandler {
	return &HealthHandler{pinger: pinger, logger: logger}
}

// Health handles GET /api/v1/health.
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	if err := h.pinger.Ping(ctx); err != nil {
		h.logger.ErrorContext(
			r.Context(),
			"database health check failed",
			"error", err,
			"request_id", requestIDFromContext(r.Context()),
		)
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "degraded", Database: "unavailable"})

		return
	}

	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Database: "ok"})
}
