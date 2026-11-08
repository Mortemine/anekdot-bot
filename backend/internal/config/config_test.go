package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func cleanEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"HTTP_ADDR",
		"DATABASE_URL",
		"DB_MAX_CONNS",
		"DB_MIN_CONNS",
		"DB_MAX_CONN_LIFETIME",
		"DB_MAX_CONN_IDLE_TIME",
		"SESSION_TTL",
		"CORS_ORIGINS",
		"TRUST_PROXY_HEADERS",
		"WEB_DIR",
		"DATA_DIR",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"EXTERNAL_ANEKDOT_URL",
		"REQUEST_TIMEOUT",
		"SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	cleanEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, defaultDatabaseURL)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
	}
	if cfg.RequestTimeout != 10*time.Second {
		t.Errorf("RequestTimeout = %v, want 10s", cfg.RequestTimeout)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Errorf("ShutdownTimeout = %v, want 15s", cfg.ShutdownTimeout)
	}
	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSOrigins = %v, want [http://localhost:5173]", cfg.CORSOrigins)
	}
	if cfg.ExternalAnekdotURL != defaultExternalURL {
		t.Errorf("ExternalAnekdotURL = %q, want %q", cfg.ExternalAnekdotURL, defaultExternalURL)
	}
}

func TestLoadInfrastructureDefaults(t *testing.T) {
	cleanEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DBMaxConns != 10 || cfg.DBMinConns != 2 {
		t.Errorf("pool sizes = (%d, %d), want (10, 2)", cfg.DBMaxConns, cfg.DBMinConns)
	}
	if cfg.DBMaxConnLifetime != 30*time.Minute {
		t.Errorf("DBMaxConnLifetime = %v, want 30m", cfg.DBMaxConnLifetime)
	}
	if cfg.DBMaxConnIdleTime != 5*time.Minute {
		t.Errorf("DBMaxConnIdleTime = %v, want 5m", cfg.DBMaxConnIdleTime)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("SessionTTL = %v, want 720h", cfg.SessionTTL)
	}
	if cfg.TrustProxyHeaders {
		t.Error("TrustProxyHeaders = true, want false")
	}
	if cfg.WebDir != "backend/web" {
		t.Errorf("WebDir = %q, want backend/web", cfg.WebDir)
	}
	if cfg.DataDir != "data" {
		t.Errorf("DataDir = %q, want data", cfg.DataDir)
	}
}

func TestLoadOverrides(t *testing.T) {
	cleanEnv(t)

	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/app?sslmode=require")
	t.Setenv("SESSION_TTL", "48h")
	t.Setenv("CORS_ORIGINS", "https://example.com/, https://app.example.com")
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("REQUEST_TIMEOUT", "3s")
	t.Setenv("SHUTDOWN_TIMEOUT", "5s")
	t.Setenv("DB_MAX_CONNS", "20")
	t.Setenv("DB_MIN_CONNS", "4")
	t.Setenv("WEB_DIR", "/srv/web")
	t.Setenv("DATA_DIR", "/srv/data")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.SessionTTL != 48*time.Hour {
		t.Errorf("SessionTTL = %v, want 48h", cfg.SessionTTL)
	}
	if !cfg.TrustProxyHeaders {
		t.Error("TrustProxyHeaders = false, want true")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want text", cfg.LogFormat)
	}
	if cfg.RequestTimeout != 3*time.Second {
		t.Errorf("RequestTimeout = %v, want 3s", cfg.RequestTimeout)
	}
	if cfg.DBMaxConns != 20 || cfg.DBMinConns != 4 {
		t.Errorf("pool sizes = (%d, %d), want (20, 4)", cfg.DBMaxConns, cfg.DBMinConns)
	}
	if cfg.WebDir != "/srv/web" {
		t.Errorf("WebDir = %q, want /srv/web", cfg.WebDir)
	}
	if cfg.DataDir != "/srv/data" {
		t.Errorf("DataDir = %q, want /srv/data", cfg.DataDir)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://example.com" {
		t.Errorf("CORSOrigins = %v", cfg.CORSOrigins)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "http address",
			env:  map[string]string{"HTTP_ADDR": "localhost"},
			want: "HTTP_ADDR",
		},
		{
			name: "database scheme",
			env:  map[string]string{"DATABASE_URL": "mysql://user:pass@localhost:3306/app"},
			want: "postgres scheme",
		},
		{
			name: "database name",
			env:  map[string]string{"DATABASE_URL": "postgres://user:pass@localhost:5432/"},
			want: "database name",
		},
		{
			name: "external url",
			env:  map[string]string{"EXTERNAL_ANEKDOT_URL": "ftp://example.com/random"},
			want: "EXTERNAL_ANEKDOT_URL",
		},
		{
			name: "pool size",
			env:  map[string]string{"DB_MAX_CONNS": "0"},
			want: "DB_MAX_CONNS",
		},
		{
			name: "min connections",
			env:  map[string]string{"DB_MIN_CONNS": "50"},
			want: "DB_MIN_CONNS",
		},
		{
			name: "log level",
			env:  map[string]string{"LOG_LEVEL": "trace"},
			want: "LOG_LEVEL",
		},
		{
			name: "log format",
			env:  map[string]string{"LOG_FORMAT": "xml"},
			want: "LOG_FORMAT",
		},
		{
			name: "session ttl",
			env:  map[string]string{"SESSION_TTL": "0s"},
			want: "SESSION_TTL",
		},
		{
			name: "cors origin",
			env:  map[string]string{"CORS_ORIGINS": "example.com"},
			want: "CORS_ORIGINS",
		},
		{
			name: "request timeout",
			env:  map[string]string{"REQUEST_TIMEOUT": "-1s"},
			want: "REQUEST_TIMEOUT",
		},
		{
			name: "shutdown timeout",
			env:  map[string]string{"SHUTDOWN_TIMEOUT": "0s"},
			want: "SHUTDOWN_TIMEOUT",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleanEnv(t)
			for key, value := range test.env {
				t.Setenv(key, value)
			}

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want error")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Load() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}
