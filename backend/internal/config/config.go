package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr        = ":8080"
	defaultDatabaseURL     = "postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable"
	defaultExternalURL     = "https://baneks.site/random"
	defaultCORSOrigins     = "http://localhost:5173"
	defaultDataDir         = "data"
	defaultRequestTimeout  = 10 * time.Second
	defaultShutdownTimeout = 15 * time.Second
	defaultSessionTTL      = 30 * 24 * time.Hour

	defaultDBMaxConns    int32 = 10
	defaultDBMinConns    int32 = 2
	defaultDBMaxConnLife       = 30 * time.Minute
	defaultDBMaxConnIdle       = 5 * time.Minute
)

type Config struct {
	HTTPAddr           string
	DatabaseURL        string
	DBMaxConns         int32
	DBMinConns         int32
	DBMaxConnLifetime  time.Duration
	DBMaxConnIdleTime  time.Duration
	SessionTTL         time.Duration
	CORSOrigins        []string
	TrustProxyHeaders  bool
	WebDir             string
	DataDir            string
	LogLevel           slog.Level
	LogFormat          string
	ExternalAnekdotURL string
	RequestTimeout     time.Duration
	ShutdownTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:           env("HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:        env("DATABASE_URL", defaultDatabaseURL),
		WebDir:             strings.TrimSpace(os.Getenv("WEB_DIR")),
		DataDir:            env("DATA_DIR", defaultDataDir),
		LogFormat:          strings.ToLower(env("LOG_FORMAT", "json")),
		ExternalAnekdotURL: env("EXTERNAL_ANEKDOT_URL", defaultExternalURL),
		TrustProxyHeaders:  envBool("TRUST_PROXY_HEADERS", false),
	}

	var err error
	if cfg.DBMaxConns, err = envInt32("DB_MAX_CONNS", defaultDBMaxConns); err != nil {
		return nil, err
	}
	if cfg.DBMinConns, err = envInt32("DB_MIN_CONNS", defaultDBMinConns); err != nil {
		return nil, err
	}
	if cfg.DBMaxConnLifetime, err = envDuration("DB_MAX_CONN_LIFETIME", defaultDBMaxConnLife); err != nil {
		return nil, err
	}
	if cfg.DBMaxConnIdleTime, err = envDuration("DB_MAX_CONN_IDLE_TIME", defaultDBMaxConnIdle); err != nil {
		return nil, err
	}
	if cfg.LogLevel, err = parseLogLevel(env("LOG_LEVEL", "info")); err != nil {
		return nil, err
	}
	if cfg.CORSOrigins, err = parseOrigins(env("CORS_ORIGINS", defaultCORSOrigins)); err != nil {
		return nil, err
	}
	if cfg.RequestTimeout, err = envDuration("REQUEST_TIMEOUT", defaultRequestTimeout); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = envDuration("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return nil, err
	}
	if cfg.SessionTTL, err = envDuration("SESSION_TTL", defaultSessionTTL); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if _, _, err := net.SplitHostPort(c.HTTPAddr); err != nil {
		return fmt.Errorf("config: HTTP_ADDR %q is not a host:port pair: %w", c.HTTPAddr, err)
	}

	if err := validatePostgresURL(c.DatabaseURL); err != nil {
		return err
	}
	if err := validateHTTPURL("EXTERNAL_ANEKDOT_URL", c.ExternalAnekdotURL); err != nil {
		return err
	}

	if c.DBMaxConns <= 0 {
		return fmt.Errorf("config: DB_MAX_CONNS must be positive")
	}
	if c.DBMinConns < 0 || c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("config: DB_MIN_CONNS must be between 0 and DB_MAX_CONNS")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("config: REQUEST_TIMEOUT must be positive")
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("config: SHUTDOWN_TIMEOUT must be positive")
	}
	if c.LogFormat != "json" && c.LogFormat != "text" {
		return fmt.Errorf("config: LOG_FORMAT must be json or text")
	}
	if c.SessionTTL <= 0 {
		return fmt.Errorf("config: SESSION_TTL must be positive")
	}

	return nil
}

func validatePostgresURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("config: DATABASE_URL is invalid: %w", err)
	}

	scheme := parsed.Scheme
	if scheme != "postgres" && scheme != "postgresql" {
		return fmt.Errorf("config: DATABASE_URL must use the postgres scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("config: DATABASE_URL is missing a host")
	}
	if strings.Trim(parsed.Path, "/") == "" {
		return fmt.Errorf("config: DATABASE_URL is missing a database name")
	}

	return nil
}

func validateHTTPURL(name, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("config: %s is invalid: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("config: %s must use the http or https scheme", name)
	}
	if parsed.Host == "" {
		return fmt.Errorf("config: %s is missing a host", name)
	}

	return nil
}

func parseOrigins(raw string) ([]string, error) {
	origins := []string{}
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin == "" {
			continue
		}
		if origin == "*" {
			origins = append(origins, origin)
			continue
		}
		if err := validateHTTPURL("CORS_ORIGINS", origin); err != nil {
			return nil, err
		}
		origins = append(origins, strings.TrimRight(origin, "/"))
	}

	return origins, nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("config: LOG_LEVEL must be debug, info, warn or error")
	}
}

func env(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}

	return fallback
}

func envBool(key string, fallback bool) bool {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}

	return parsed
}

func envInt32(key string, fallback int32) (int32, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}

	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("config: %s must be an integer: %w", key, err)
	}

	return int32(value), nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}

	value, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("config: %s must be a duration: %w", key, err)
	}

	return value, nil
}
