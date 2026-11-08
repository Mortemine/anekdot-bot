// Package httpapi implements the driving HTTP adapter.
package httpapi

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

const (
	feedbackRateWindow = time.Minute
	feedbackRateBurst  = 5
)

// NewRouter builds the HTTP handler tree with all middleware applied.
func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	anekdotService *usecase.AnekdotService,
	feedbackService *usecase.FeedbackService,
	pinger databasePinger,
) (http.Handler, error) {
	mux := http.NewServeMux()

	anekdotHandler := NewAnekdotHandler(anekdotService, logger)
	feedbackHandler := NewFeedbackHandler(feedbackService, logger)
	healthHandler := NewHealthHandler(pinger, logger)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Health)
	mux.HandleFunc("GET /api/v1/categories", anekdotHandler.ListCategories)
	mux.HandleFunc("GET /api/v1/anekdots/random", anekdotHandler.Random)
	mux.HandleFunc("GET /api/v1/anekdots/external", anekdotHandler.External)

	feedbackLimiter := newIPRateLimiter(
		rate.Every(feedbackRateWindow/feedbackRateBurst),
		feedbackRateBurst,
	)
	mux.Handle(
		"POST /api/v1/feedback",
		rateLimitMiddleware(feedbackLimiter, cfg.TrustProxyHeaders, logger)(http.HandlerFunc(feedbackHandler.Create)),
	)

	if cfg.AdminEnabled() {
		mux.Handle(
			"GET /api/v1/admin/feedback",
			requireAdmin(cfg.AdminToken)(http.HandlerFunc(feedbackHandler.List)),
		)
	} else {
		logger.Warn("admin API disabled: ADMIN_TOKEN is not set")
	}

	mux.HandleFunc("GET /api/v1/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "Эндпоинт не найден.")
	})

	if cfg.WebDir != "" {
		staticHandler, err := newSPAHandler(cfg.WebDir)
		if err != nil {
			return nil, err
		}
		mux.Handle("GET /", staticHandler)
	} else {
		logger.Info("static frontend disabled: WEB_DIR is not set")
	}

	return chain(
		mux,
		requestIDMiddleware,
		accessLogMiddleware(logger),
		recovererMiddleware(logger),
		securityHeadersMiddleware,
		corsMiddleware(cfg.CORSOrigins),
	), nil
}

func newSPAHandler(directory string) (http.Handler, error) {
	info, err := os.Stat(directory)
	if err != nil {
		return nil, fmt.Errorf("checking web directory %q: %w", directory, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("web directory %q is not a directory", directory)
	}

	fileSystem := os.DirFS(directory)
	files := http.FileServerFS(fileSystem)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if requestPath == "" {
			requestPath = "index.html"
		}

		info, err := fs.Stat(fileSystem, requestPath)
		if err != nil || info.IsDir() {
			requestPath = "index.html"
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		if strings.HasPrefix(requestPath, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		files.ServeHTTP(w, r)
	}), nil
}
