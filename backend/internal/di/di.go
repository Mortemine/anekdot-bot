// Package di wires every layer of the application.
package di

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"

	"github.com/alexey/anekdot-bot/backend/internal/adapter/external"
	"github.com/alexey/anekdot-bot/backend/internal/adapter/httpapi"
	"github.com/alexey/anekdot-bot/backend/internal/adapter/postgres"
	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

// New builds the dependency injection container and eagerly validates the graph.
func New(cfg *config.Config, logger *slog.Logger) (*do.RootScope, error) {
	injector := do.New()

	do.ProvideValue(injector, cfg)
	do.ProvideValue(injector, logger)

	do.Provide(injector, func(i do.Injector) (*pgxpool.Pool, error) {
		return postgres.NewPool(
			context.Background(),
			do.MustInvoke[*config.Config](i),
			do.MustInvoke[*slog.Logger](i),
		)
	})

	do.Provide(injector, func(i do.Injector) (domain.CategoryRepository, error) {
		return postgres.NewCategoryRepository(do.MustInvoke[*pgxpool.Pool](i)), nil
	})
	do.Provide(injector, func(i do.Injector) (domain.AnekdotRepository, error) {
		return postgres.NewAnekdotRepository(do.MustInvoke[*pgxpool.Pool](i)), nil
	})
	do.Provide(injector, func(i do.Injector) (domain.UserRepository, error) {
		return postgres.NewUserRepository(do.MustInvoke[*pgxpool.Pool](i)), nil
	})
	do.Provide(injector, func(i do.Injector) (domain.SessionRepository, error) {
		return postgres.NewSessionRepository(do.MustInvoke[*pgxpool.Pool](i)), nil
	})
	do.Provide(injector, func(i do.Injector) (domain.FavoriteRepository, error) {
		return postgres.NewFavoriteRepository(do.MustInvoke[*pgxpool.Pool](i)), nil
	})

	do.Provide(injector, func(i do.Injector) (domain.ExternalAnekdotSource, error) {
		settings := do.MustInvoke[*config.Config](i)

		return external.NewBaneksSource(
			settings.ExternalAnekdotURL,
			settings.RequestTimeout,
			do.MustInvoke[*slog.Logger](i),
		)
	})

	do.Provide(injector, func(i do.Injector) (*usecase.AnekdotService, error) {
		return usecase.NewAnekdotService(
			do.MustInvoke[domain.CategoryRepository](i),
			do.MustInvoke[domain.AnekdotRepository](i),
			do.MustInvoke[domain.FavoriteRepository](i),
			do.MustInvoke[domain.ExternalAnekdotSource](i),
			do.MustInvoke[*slog.Logger](i),
		), nil
	})
	do.Provide(injector, func(i do.Injector) (*usecase.AuthService, error) {
		return usecase.NewAuthService(
			do.MustInvoke[domain.UserRepository](i),
			do.MustInvoke[domain.SessionRepository](i),
			do.MustInvoke[*config.Config](i).SessionTTL,
			do.MustInvoke[*slog.Logger](i),
		), nil
	})

	do.Provide(injector, func(i do.Injector) (http.Handler, error) {
		return httpapi.NewRouter(
			do.MustInvoke[*config.Config](i),
			do.MustInvoke[*slog.Logger](i),
			do.MustInvoke[*usecase.AnekdotService](i),
			do.MustInvoke[*usecase.AuthService](i),
			do.MustInvoke[*pgxpool.Pool](i),
		)
	})
	do.Provide(injector, func(i do.Injector) (*http.Server, error) {
		return httpapi.NewServer(
			do.MustInvoke[*config.Config](i),
			do.MustInvoke[http.Handler](i),
		), nil
	})

	if _, err := do.Invoke[*http.Server](injector); err != nil {
		return nil, fmt.Errorf("initializing services: %w", err)
	}

	return injector, nil
}
