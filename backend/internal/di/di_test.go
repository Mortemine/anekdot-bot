package di

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/alexey/anekdotus/backend/internal/config"
)

func TestNewFailsWithInvalidDatabaseURL(t *testing.T) {
	cfg := &config.Config{
		DatabaseURL: "://invalid",
		WebDir:      t.TempDir(),
	}

	scope, err := New(cfg, slog.New(slog.DiscardHandler))

	if err == nil {
		t.Fatal("expected an error for an invalid database url")
	}
	if scope != nil {
		t.Fatalf("expected a nil scope, got %v", scope)
	}
	if !strings.Contains(err.Error(), "initializing services") {
		t.Fatalf("unexpected error: %v", err)
	}
}
