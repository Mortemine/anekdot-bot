package config

import (
	"log/slog"
	"os"
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

func TestLoadRejectsUnparsableEnv(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "max connections", key: "DB_MAX_CONNS", want: "DB_MAX_CONNS"},
		{name: "min connections", key: "DB_MIN_CONNS", want: "DB_MIN_CONNS"},
		{name: "connection lifetime", key: "DB_MAX_CONN_LIFETIME", want: "DB_MAX_CONN_LIFETIME"},
		{name: "connection idle time", key: "DB_MAX_CONN_IDLE_TIME", want: "DB_MAX_CONN_IDLE_TIME"},
		{name: "request timeout", key: "REQUEST_TIMEOUT", want: "REQUEST_TIMEOUT"},
		{name: "shutdown timeout", key: "SHUTDOWN_TIMEOUT", want: "SHUTDOWN_TIMEOUT"},
		{name: "session ttl", key: "SESSION_TTL", want: "SESSION_TTL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cleanEnv(t)
			t.Setenv(test.key, "not-a-value")

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() error = nil, want it to reject %s", test.key)
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Errorf("Load() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func validConfig() Config {
	return Config{
		HTTPAddr:           ":8080",
		DatabaseURL:        "postgres://user:pass@localhost:5432/app",
		DBMaxConns:         10,
		DBMinConns:         0,
		RequestTimeout:     time.Nanosecond,
		ShutdownTimeout:    time.Nanosecond,
		SessionTTL:         time.Nanosecond,
		LogFormat:          "json",
		ExternalAnekdotURL: "https://example.com/random",
	}
}

func TestValidateBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "baseline"},
		{name: "lowest pool", mutate: func(c *Config) { c.DBMaxConns = 1 }},
		{name: "no idle connections", mutate: func(c *Config) { c.DBMinConns = 0 }},
		{name: "pool fully reserved", mutate: func(c *Config) { c.DBMinConns = c.DBMaxConns }},
		{name: "postgresql scheme", mutate: func(c *Config) { c.DatabaseURL = "postgresql://u:p@db:5432/app" }},
		{name: "trailing slash", mutate: func(c *Config) { c.DatabaseURL = "postgres://u:p@db:5432/app/" }},
		{name: "root address", mutate: func(c *Config) { c.HTTPAddr = ":0" }},
		{name: "text logs", mutate: func(c *Config) { c.LogFormat = "text" }},
		{name: "http external", mutate: func(c *Config) { c.ExternalAnekdotURL = "http://example.com/random" }},
		{name: "port only", mutate: func(c *Config) { c.ExternalAnekdotURL = "https://example.com" }},
		{
			name:   "missing database name",
			mutate: func(c *Config) { c.DatabaseURL = "postgres://u:p@localhost:5432" },
			want:   "database name",
		},
		{
			name:   "missing database host",
			mutate: func(c *Config) { c.DatabaseURL = "postgres:///app" },
			want:   "host",
		},
		{
			name:   "negative pool",
			mutate: func(c *Config) { c.DBMaxConns = -1 },
			want:   "DB_MAX_CONNS",
		},
		{
			name:   "negative minimum",
			mutate: func(c *Config) { c.DBMinConns = -1 },
			want:   "DB_MIN_CONNS",
		},
		{
			name:   "no external host",
			mutate: func(c *Config) { c.ExternalAnekdotURL = "https:///random" },
			want:   "host",
		},
		{
			name:   "empty log format",
			mutate: func(c *Config) { c.LogFormat = "" },
			want:   "LOG_FORMAT",
		},
		{
			name:   "negative session ttl",
			mutate: func(c *Config) { c.SessionTTL = -time.Hour },
			want:   "SESSION_TTL",
		},
		{
			name:   "zero pool size",
			mutate: func(c *Config) { c.DBMaxConns = 0 },
			want:   "DB_MAX_CONNS",
		},
		{
			name:   "zero request timeout",
			mutate: func(c *Config) { c.RequestTimeout = 0 },
			want:   "REQUEST_TIMEOUT",
		},
		{
			name:   "zero shutdown timeout",
			mutate: func(c *Config) { c.ShutdownTimeout = 0 },
			want:   "SHUTDOWN_TIMEOUT",
		},
		{
			name:   "zero session ttl",
			mutate: func(c *Config) { c.SessionTTL = 0 },
			want:   "SESSION_TTL",
		},
		{
			name:   "minimum above maximum",
			mutate: func(c *Config) { c.DBMinConns = c.DBMaxConns + 1 },
			want:   "DB_MIN_CONNS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			if test.mutate != nil {
				test.mutate(&cfg)
			}

			err := cfg.validate()
			if test.want == "" {
				if err != nil {
					t.Fatalf("validate() error = %v, want nil", err)
				}

				return
			}

			if err == nil {
				t.Fatal("validate() error = nil, want error")
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validate() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		raw     string
		want    slog.Level
		wantErr bool
	}{
		{raw: "debug", want: slog.LevelDebug},
		{raw: "INFO", want: slog.LevelInfo},
		{raw: " info ", want: slog.LevelInfo},
		{raw: "warn", want: slog.LevelWarn},
		{raw: "warning", want: slog.LevelWarn},
		{raw: "error", want: slog.LevelError},
		{raw: "Error", want: slog.LevelError},
		{raw: "trace", wantErr: true},
		{raw: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			got, err := parseLogLevel(test.raw)

			if test.wantErr {
				if err == nil {
					t.Fatalf("parseLogLevel(%q) error = nil, want error", test.raw)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseLogLevel(%q) error = %v", test.raw, err)
			}

			if got != test.want {
				t.Fatalf("parseLogLevel(%q) = %v, want %v", test.raw, got, test.want)
			}
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback bool
		want     bool
	}{
		{name: "true", value: "true", fallback: false, want: true},
		{name: "one", value: "1", fallback: false, want: true},
		{name: "upper case", value: "TRUE", fallback: false, want: true},
		{name: "false overrides true fallback", value: "false", fallback: true, want: false},
		{name: "zero", value: "0", fallback: true, want: false},
		{name: "empty takes the fallback", value: "", fallback: true, want: true},
		{name: "blank takes the fallback", value: "  ", fallback: false, want: false},
		{name: "garbage takes the fallback", value: "maybe", fallback: true, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			const key = "ANEKDOT_TEST_FLAG"

			if test.value != "" {
				t.Setenv(key, test.value)
			}

			if got := envBool(key, test.fallback); got != test.want {
				t.Fatalf("envBool(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}

	t.Run("absent takes the fallback", func(t *testing.T) {
		if envBool("ANEKDOT_TEST_UNSET_FLAG", false) {
			t.Fatal("envBool() = true for an absent variable with a false fallback")
		}

		if !envBool("ANEKDOT_TEST_UNSET_FLAG", true) {
			t.Fatal("envBool() = false for an absent variable with a true fallback")
		}
	})
}

func TestEnvReadsTrimmedValue(t *testing.T) {
	const key = "ANEKDOT_TEST_TEXT"

	t.Setenv(key, "  value  ")

	if got := env(key, "fallback"); got != "value" {
		t.Fatalf("env() = %q, want value", got)
	}

	t.Setenv(key, "   ")

	if got := env(key, "fallback"); got != "fallback" {
		t.Fatalf("env() = %q, want fallback", got)
	}
}

func TestEnvDurationRejectsGarbage(t *testing.T) {
	const key = "ANEKDOT_TEST_DURATION"

	t.Setenv(key, "later")

	value, err := envDuration(key, time.Second)
	if err == nil {
		t.Fatal("envDuration() error = nil, want error")
	}

	if value != 0 {
		t.Fatalf("envDuration() = %v, want zero on error", value)
	}

	t.Setenv(key, " 2s ")

	value, err = envDuration(key, time.Second)
	if err != nil {
		t.Fatalf("envDuration() error = %v", err)
	}

	if value != 2*time.Second {
		t.Fatalf("envDuration() = %v, want 2s", value)
	}
}

func TestEnvInt32RejectsGarbage(t *testing.T) {
	const key = "ANEKDOT_TEST_INT"

	t.Setenv(key, "many")

	value, err := envInt32(key, 3)
	if err == nil {
		t.Fatal("envInt32() error = nil, want error")
	}

	if value != 0 {
		t.Fatalf("envInt32() = %d, want zero on error", value)
	}

	t.Setenv(key, " 7 ")

	value, err = envInt32(key, 3)
	if err != nil {
		t.Fatalf("envInt32() error = %v", err)
	}

	if value != 7 {
		t.Fatalf("envInt32() = %d, want 7", value)
	}
}

func TestEnvHelpersFallBackForBlankValues(t *testing.T) {
	const key = "ANEKDOT_TEST_BLANK"

	t.Setenv(key, "   ")

	if got := envBool(key, true); !got {
		t.Fatalf("envBool() = %v, want the fallback", got)
	}

	value, err := envInt32(key, 5)
	if err != nil {
		t.Fatalf("envInt32() error = %v", err)
	}

	if value != 5 {
		t.Fatalf("envInt32() = %d, want the fallback", value)
	}

	duration, err := envDuration(key, time.Minute)
	if err != nil {
		t.Fatalf("envDuration() error = %v", err)
	}

	if duration != time.Minute {
		t.Fatalf("envDuration() = %v, want the fallback", duration)
	}

	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetting %s: %v", key, err)
	}

	if got := envBool(key, false); got {
		t.Fatalf("envBool() = %v, want the fallback for an unset key", got)
	}
}

func TestParseOrigins(t *testing.T) {
	tests := []struct {
		raw     string
		want    []string
		wantErr bool
	}{
		{raw: "", want: []string{}},
		{raw: " , ", want: []string{}},
		{raw: "*", want: []string{"*"}},
		{raw: "https://a.example", want: []string{"https://a.example"}},
		{raw: "https://a.example/, https://b.example", want: []string{"https://a.example", "https://b.example"}},
		{raw: "http://localhost:5173", want: []string{"http://localhost:5173"}},
		{raw: "example.com", wantErr: true},
		{raw: "https://a.example, ftp://b.example", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			got, err := parseOrigins(test.raw)

			if test.wantErr {
				if err == nil {
					t.Fatalf("parseOrigins(%q) error = nil, want error", test.raw)
				}

				return
			}

			if err != nil {
				t.Fatalf("parseOrigins(%q) error = %v", test.raw, err)
			}

			if len(got) != len(test.want) {
				t.Fatalf("parseOrigins(%q) = %v, want %v", test.raw, got, test.want)
			}

			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("parseOrigins(%q) = %v, want %v", test.raw, got, test.want)
				}
			}
		})
	}
}
