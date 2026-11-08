// Package app runs the HTTP server lifecycle.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"

	"github.com/alexey/anekdot-bot/backend/internal/config"
)

// Run starts the HTTP server and blocks until a shutdown signal arrives.
func Run(ctx context.Context, cfg *config.Config, injector *do.RootScope, logger *slog.Logger) error {
	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := do.Invoke[*pgxpool.Pool](injector)
	if err != nil {
		return fmt.Errorf("resolving database pool: %w", err)
	}
	defer pool.Close()

	server, err := do.Invoke[*http.Server](injector)
	if err != nil {
		return fmt.Errorf("resolving http server: %w", err)
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	logger.Info("http server started", "addr", cfg.HTTPAddr, "web_dir", cfg.WebDir)

	select {
	case serveErr := <-serverErrors:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serving http: %w", serveErr)
		}

		return nil
	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down http server: %w", err)
	}

	logger.Info("http server stopped")

	return nil
}
