package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/alexey/anekdotus/backend/internal/config"
	"github.com/alexey/anekdotus/backend/internal/domain"
)

func TestWriteJSONRejectsUnsupportedPayload(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeJSON(recorder, http.StatusOK, make(chan int))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

func TestDecodeRequestRejects(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
	}{
		{name: "empty body", body: "", code: http.StatusBadRequest},
		{name: "unknown field", body: `{"email":"a@b.ru","extra":1}`, code: http.StatusBadRequest},
		{name: "trailing object", body: `{"email":"a@b.ru"}{"email":"c@d.ru"}`, code: http.StatusBadRequest},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(
				context.Background(),
				http.MethodPost,
				"/",
				strings.NewReader(test.body),
			)
			recorder := httptest.NewRecorder()

			var payload loginRequest
			if decodeRequest(recorder, request, maxAuthBodyBytes, &payload) {
				t.Fatal("decodeRequest() = true, want false")
			}

			if recorder.Code != test.code {
				t.Fatalf("status = %d, want %d", recorder.Code, test.code)
			}
		})
	}
}

func TestStatusRecorderUnwrap(t *testing.T) {
	base := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: base}

	if recorder.Unwrap() != base {
		t.Fatal("Unwrap() does not return the wrapped writer")
	}
}

func TestRequireUserWithoutUser(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	user, ok := requireUser(recorder, request)
	if ok || user.ID != 0 {
		t.Fatalf("requireUser() = %+v, %v, want zero user and false", user, ok)
	}

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestWriteRequestError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("unexpected error is logged and hidden", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		recorder := httptest.NewRecorder()

		writeRequestError(recorder, request, logger, "test failed", errors.New("boom"))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}

		if strings.Contains(recorder.Body.String(), "boom") {
			t.Fatalf("body leaks the internal error: %s", recorder.Body.String())
		}
	})

	t.Run("unauthorized advertises the scheme", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		recorder := httptest.NewRecorder()

		writeRequestError(recorder, request, logger, "test failed", domain.ErrUnauthorized)

		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}

		if got := recorder.Header().Get("WWW-Authenticate"); got == "" {
			t.Fatal("WWW-Authenticate header is missing")
		}
	})
}

func TestIPRateLimiterEvictsStaleEntries(t *testing.T) {
	limiter := newIPRateLimiter(rate.Every(time.Hour), 1)
	limiter.maxEntries = 1
	limiter.entryTTL = time.Minute
	limiter.entries["stale"] = &ipLimiterEntry{
		limiter:  rate.NewLimiter(limiter.limit, limiter.burst),
		lastSeen: time.Now().Add(-time.Hour),
	}

	if !limiter.Allow("fresh") {
		t.Fatal("Allow() = false, want true")
	}

	if _, ok := limiter.entries["stale"]; ok {
		t.Fatal("stale entry was not evicted")
	}
}

func TestNewServerUsesConfiguredAddress(t *testing.T) {
	cfg := &config.Config{HTTPAddr: ":9090"}

	server := NewServer(cfg, http.NewServeMux())

	if server.Addr != ":9090" {
		t.Errorf("addr = %q, want :9090", server.Addr)
	}

	if server.ReadHeaderTimeout != serverReadHeaderTimeout {
		t.Errorf("read header timeout = %v", server.ReadHeaderTimeout)
	}
}
