package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const seedTestDSN = "postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable"

func setBaseEnv(t *testing.T) {
	t.Helper()

	for key, value := range map[string]string{
		"HTTP_ADDR":             ":8080",
		"DATABASE_URL":          "postgres://user:pass@localhost:5432/app",
		"DB_MAX_CONNS":          "4",
		"DB_MIN_CONNS":          "0",
		"DB_MAX_CONN_LIFETIME":  "30m",
		"DB_MAX_CONN_IDLE_TIME": "5m",
		"REQUEST_TIMEOUT":       "10s",
		"SHUTDOWN_TIMEOUT":      "15s",
		"SESSION_TTL":           "720h",
		"LOG_FORMAT":            "json",
		"LOG_LEVEL":             "info",
		"WEB_DIR":               "backend/web",
		"DATA_DIR":              "data",
		"EXTERNAL_ANEKDOT_URL":  "https://example.com/random",
		"CORS_ORIGINS":          "http://localhost:5173",
		"TRUST_PROXY_HEADERS":   "false",
	} {
		t.Setenv(key, value)
	}
}

func seedDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}

	return seedTestDSN
}

func TestRunPropagatesInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "wrong scheme",
			env:  map[string]string{"DATABASE_URL": "mysql://user:pass@localhost:3306/app"},
			want: "DATABASE_URL",
		},
		{
			name: "missing host",
			env: map[string]string{
				"DATABASE_URL": "postgres:///app",
				"DATA_DIR":     t.TempDir(),
			},
			want: "host",
		},
		{
			name: "broken log format",
			env: map[string]string{
				"DATABASE_URL": "postgres://user:pass@localhost:5432/app",
				"LOG_FORMAT":   "xml",
				"DATA_DIR":     t.TempDir(),
			},
			want: "LOG_FORMAT",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setBaseEnv(t)

			for key, value := range test.env {
				t.Setenv(key, value)
			}

			err := run()
			if err == nil {
				t.Fatal("run() error = nil, want error")
			}

			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("run() error = %q, want it to mention %q", err, test.want)
			}
		})
	}
}

func TestRunReportsUnreachableDatabase(t *testing.T) {
	if os.Getenv("ANEKDOT_TEST_UNREACHABLE_POOL") == "" {
		t.Skip("set ANEKDOT_TEST_UNREACHABLE_POOL to run the pool failure check")
	}

	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/app?sslmode=disable&connect_timeout=1")
	t.Setenv("DATA_DIR", t.TempDir())

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "pinging database") {
		t.Fatalf("run() error = %q, want it to mention pinging", err)
	}
}

func TestRunReportsMissingDataDirectory(t *testing.T) {
	dsn := seedDSN()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("skipping database test: %v", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()

		t.Skipf("skipping database test: %v", err)
	}

	pool.Close()

	setBaseEnv(t)
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("DATA_DIR", t.TempDir())

	err = run()
	if err == nil {
		t.Fatal("run() error = nil, want an error about the data files")
	}

	if !strings.Contains(err.Error(), "categories file") {
		t.Fatalf("run() error = %q, want it to mention the categories file", err)
	}
}
