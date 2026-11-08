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
	authRateWindow  = time.Minute
	authRateBurst   = 10
	writeRateWindow = time.Minute
	writeRateBurst  = 10
)

func NewRouter(
	cfg *config.Config,
	logger *slog.Logger,
	anekdotService *usecase.AnekdotService,
	authService *usecase.AuthService,
	pinger databasePinger,
) (http.Handler, error) {
	mux := http.NewServeMux()

	anekdotHandler := NewAnekdotHandler(anekdotService, logger)
	likeHandler := NewLikeHandler(anekdotService, logger)
	favoriteHandler := NewFavoriteHandler(anekdotService, logger)
	authHandler := NewAuthHandler(authService, logger)
	healthHandler := NewHealthHandler(pinger, logger)

	requireAuth := authMiddleware(authService, logger)

	mux.HandleFunc("GET /api/v1/health", healthHandler.Health)
	mux.HandleFunc("GET /api/v1/categories", anekdotHandler.ListCategories)
	mux.HandleFunc("GET /api/v1/anekdots", anekdotHandler.List)
	mux.HandleFunc("GET /api/v1/anekdots/random", anekdotHandler.Random)
	mux.HandleFunc("GET /api/v1/anekdots/external", anekdotHandler.External)
	mux.Handle("GET /api/v1/anekdots/mine", requireAuth(http.HandlerFunc(anekdotHandler.Mine)))

	authLimiter := newIPRateLimiter(rate.Every(authRateWindow/authRateBurst), authRateBurst)
	mux.Handle(
		"POST /api/v1/auth/register",
		rateLimitMiddleware(authLimiter, cfg.TrustProxyHeaders, logger)(http.HandlerFunc(authHandler.Register)),
	)
	mux.Handle(
		"POST /api/v1/auth/login",
		rateLimitMiddleware(authLimiter, cfg.TrustProxyHeaders, logger)(http.HandlerFunc(authHandler.Login)),
	)
	mux.Handle("POST /api/v1/auth/logout", requireAuth(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("GET /api/v1/auth/me", requireAuth(http.HandlerFunc(authHandler.Me)))

	writeLimiter := newIPRateLimiter(rate.Every(writeRateWindow/writeRateBurst), writeRateBurst)
	writeLimited := rateLimitMiddleware(writeLimiter, cfg.TrustProxyHeaders, logger)
	mux.Handle(
		"POST /api/v1/anekdots",
		requireAuth(writeLimited(http.HandlerFunc(anekdotHandler.Create))),
	)

	mux.Handle("GET /api/v1/likes", requireAuth(http.HandlerFunc(likeHandler.List)))
	mux.Handle("PUT /api/v1/likes/{anekdot_id}", requireAuth(http.HandlerFunc(likeHandler.Add)))
	mux.Handle("DELETE /api/v1/likes/{anekdot_id}", requireAuth(http.HandlerFunc(likeHandler.Remove)))

	mux.Handle("GET /api/v1/favorites", requireAuth(http.HandlerFunc(favoriteHandler.List)))
	mux.Handle("PUT /api/v1/favorites/{anekdot_id}", requireAuth(http.HandlerFunc(favoriteHandler.Add)))
	mux.Handle("DELETE /api/v1/favorites/{anekdot_id}", requireAuth(http.HandlerFunc(favoriteHandler.Remove)))

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
