package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"

	"github.com/alexey/anekdotus/backend/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()

	return &config.Config{
		HTTPAddr:        freeAddr(t),
		DatabaseURL:     "postgres://user:pass@127.0.0.1:1/app?sslmode=disable",
		WebDir:          "backend/web",
		ShutdownTimeout: 2 * time.Second,
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()

	var listenConfig net.ListenConfig

	listener, err := listenConfig.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	addr := listener.Addr().String()

	if err := listener.Close(); err != nil {
		t.Fatalf("closing listener: %v", err)
	}

	return addr
}

func testPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()

	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	return pool
}

func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestRunReportsMissingPool(t *testing.T) {
	err := Run(context.Background(), testConfig(t), do.New(), testLogger())
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "resolving database pool") {
		t.Fatalf("Run() error = %q, want it to mention the pool", err)
	}
}

func TestRunReportsMissingServer(t *testing.T) {
	cfg := testConfig(t)
	injector := do.New()
	do.ProvideValue(injector, testPool(t, cfg.DatabaseURL))

	err := Run(context.Background(), cfg, injector, testLogger())
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}

	if !strings.Contains(err.Error(), "resolving http server") {
		t.Fatalf("Run() error = %q, want it to mention the server", err)
	}
}

func TestRunReportsListenFailure(t *testing.T) {
	cfg := testConfig(t)
	cfg.HTTPAddr = "not-a-host-port"

	injector := do.New()
	do.ProvideValue(injector, testPool(t, cfg.DatabaseURL))
	do.ProvideValue(injector, &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	})

	err := Run(context.Background(), cfg, injector, testLogger())
	if err == nil {
		t.Fatal("Run() error = nil, want a serve error")
	}

	if !strings.Contains(err.Error(), "serving http") {
		t.Fatalf("Run() error = %q, want it to mention serving", err)
	}
}

func TestServeResult(t *testing.T) {
	if err := serveResult(nil); err != nil {
		t.Fatalf("serveResult(nil) = %v, want nil", err)
	}

	if err := serveResult(http.ErrServerClosed); err != nil {
		t.Fatalf("serveResult(ErrServerClosed) = %v, want nil", err)
	}

	err := serveResult(errors.New("boom"))
	if err == nil || !strings.Contains(err.Error(), "serving http") {
		t.Fatalf("serveResult(boom) = %v, want it to mention serving", err)
	}
}

func TestRunStopsOnSignal(t *testing.T) {
	cfg := testConfig(t)

	injector := do.New()
	do.ProvideValue(injector, testPool(t, cfg.DatabaseURL))
	do.ProvideValue(injector, &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: time.Second,
	})

	done := make(chan error, 1)
	go func() { done <- Run(context.Background(), cfg, injector, testLogger()) }()

	waitForListener(t, cfg.HTTPAddr)

	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatalf("sending signal: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %q, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run() did not stop after the signal")
	}

	waitForNoServerGoroutine(t)
}

func hasServerGoroutine() bool {
	buffer := make([]byte, 1<<20)

	return strings.Contains(string(buffer[:runtime.Stack(buffer, true)]), "app.Run.func1")
}

func waitForNoServerGoroutine(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		if !hasServerGoroutine() {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("the server goroutine is still running after Run returned")
}

func waitForListener(t *testing.T, addr string) {
	t.Helper()

	dialer := &net.Dialer{Timeout: 50 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)

	for time.Now().Before(deadline) {
		conn, err := dialer.DialContext(context.Background(), "tcp", addr)
		if err == nil {
			_ = conn.Close()

			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("listener on %s did not start", addr)
}
