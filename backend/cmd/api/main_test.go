package main

import (
	"strings"
	"testing"
)

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

func TestRunPropagatesInvalidConfig(t *testing.T) {
	for _, test := range []struct {
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
			name: "broken log format",
			env:  map[string]string{"LOG_FORMAT": "xml"},
			want: "LOG_FORMAT",
		},
		{
			name: "broken external url",
			env:  map[string]string{"EXTERNAL_ANEKDOT_URL": "not-a-url"},
			want: "EXTERNAL_ANEKDOT_URL",
		},
	} {
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
	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/app")

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want a ping error")
	}

	if !strings.Contains(err.Error(), "pinging database") {
		t.Fatalf("run() error = %q, want it to mention pinging", err)
	}
}
