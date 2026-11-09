package httpapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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
		{
			name: "oversized body",
			body: `{"email":"` + strings.Repeat("a", maxAuthBodyBytes+1) + `"}`,
			code: http.StatusRequestEntityTooLarge,
		},
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

func TestStatusRecorderAssumesOKOnWrite(t *testing.T) {
	base := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: base}

	if _, err := recorder.Write([]byte("payload")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if recorder.status != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.status)
	}

	if recorder.bytes != len("payload") {
		t.Fatalf("bytes = %d, want %d", recorder.bytes, len("payload"))
	}
}

func TestIPRateLimiterStopsAtCapacity(t *testing.T) {
	limiter := newIPRateLimiter(rate.Every(time.Hour), 1)
	limiter.maxEntries = 1
	limiter.entryTTL = time.Hour

	if !limiter.Allow("first") {
		t.Fatal("first Allow() = false, want true")
	}

	if limiter.Allow("second") {
		t.Fatal("Allow() = true for a full bucket of entries, want false")
	}

	if _, ok := limiter.entries["first"]; !ok {
		t.Fatal("a live entry was evicted")
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
		var logs bytes.Buffer

		writeRequestError(recorder, request, slog.New(slog.NewTextHandler(&logs, nil)), "test failed", errors.New("boom"))

		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}

		if strings.Contains(recorder.Body.String(), "boom") {
			t.Fatalf("body leaks the internal error: %s", recorder.Body.String())
		}

		if !strings.Contains(logs.String(), "boom") {
			t.Fatalf("unexpected error was not logged: %s", logs.String())
		}
	})

	t.Run("known error is not logged", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		recorder := httptest.NewRecorder()
		var logs bytes.Buffer

		writeRequestError(recorder, request, slog.New(slog.NewTextHandler(&logs, nil)), "test failed", domain.ErrNotFound)

		if logs.Len() != 0 {
			t.Fatalf("mapped error was logged: %s", logs.String())
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

func TestParsePathID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
		ok   bool
	}{
		{name: "one", raw: "1", want: 1, ok: true},
		{name: "large", raw: "9223372036854775807", want: 9223372036854775807, ok: true},
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-3"},
		{name: "not a number", raw: "abc"},
		{name: "empty", raw: ""},
		{name: "overflow", raw: "9223372036854775808"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			request.SetPathValue("anekdotId", test.raw)
			recorder := httptest.NewRecorder()

			got, ok := parsePathID(recorder, request, "anekdotId", "анекдота")

			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if got != test.want {
				t.Fatalf("id = %d, want %d", got, test.want)
			}

			if ok {
				return
			}

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}

			if !strings.Contains(recorder.Body.String(), "Идентификатор анекдота должен быть положительным числом.") {
				t.Fatalf("body does not name the resource: %s", recorder.Body.String())
			}
		})
	}
}

func TestParseFilterID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int64
		ok   bool
	}{
		{name: "absent", ok: true},
		{name: "valid", raw: "7", want: 7, ok: true},
		{name: "one", raw: "1", want: 1, ok: true},
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-1"},
		{name: "garbage", raw: "seven"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			got, ok := parseFilterID(recorder, test.raw, "коллекции")

			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if got != test.want {
				t.Fatalf("id = %d, want %d", got, test.want)
			}

			if !ok && recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}

			if !ok && !strings.Contains(recorder.Body.String(), "Идентификатор коллекции должен быть положительным числом.") {
				t.Fatalf("body lacks the message: %s", recorder.Body.String())
			}
		})
	}
}

func TestParsePageParam(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
		ok   bool
	}{
		{name: "absent", ok: true},
		{name: "zero", raw: "0", ok: true},
		{name: "valid", raw: "25", want: 25, ok: true},
		{name: "negative", raw: "-1"},
		{name: "garbage", raw: "ten"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			got, ok := parsePageParam(recorder, test.raw)

			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if got != test.want {
				t.Fatalf("value = %d, want %d", got, test.want)
			}

			if !ok && recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
		})
	}
}

func TestParseCategoryID(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		want     int
		filtered bool
		wantErr  bool
	}{
		{name: "absent"},
		{name: "all categories", raw: "0", filtered: true},
		{name: "external", raw: "46", want: 46, filtered: true},
		{name: "negative", raw: "-1", wantErr: true},
		{name: "garbage", raw: "external", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, filtered, err := parseCategoryID(test.raw)

			if test.wantErr {
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Fatalf("err = %v, want ErrInvalidInput", err)
				}

				if got != 0 || filtered {
					t.Fatalf("got (%d, %v), want (0, false) on error", got, filtered)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != test.want {
				t.Fatalf("category = %d, want %d", got, test.want)
			}

			if filtered != test.filtered {
				t.Fatalf("filtered = %v, want %v", filtered, test.filtered)
			}
		})
	}
}

func TestParseAnekdotSortParam(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want domain.AnekdotSort
		ok   bool
	}{
		{name: "default", want: domain.AnekdotSortCreated, ok: true},
		{name: "created", raw: "created", want: domain.AnekdotSortCreated, ok: true},
		{name: "likes", raw: "likes", want: domain.AnekdotSortLikes, ok: true},
		{name: "favorites", raw: "favorites", want: domain.AnekdotSortFavorites, ok: true},
		{name: "unknown", raw: "popular"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()

			got, ok := parseAnekdotSort(recorder, test.raw)

			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if ok && got != test.want {
				t.Fatalf("sort = %q, want %q", got, test.want)
			}

			if !ok && recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
		})
	}
}

func TestCurrentUserID(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)

	if got := currentUserID(request); got != 0 {
		t.Fatalf("anonymous id = %d, want 0", got)
	}

	ctx := context.WithValue(request.Context(), userContextKey{}, domain.User{ID: 42})
	authorized := request.WithContext(ctx)

	if got := currentUserID(authorized); got != 42 {
		t.Fatalf("authorized id = %d, want 42", got)
	}
}

func TestNewRequestID(t *testing.T) {
	seen := make(map[string]struct{}, 100)

	for range 100 {
		id := newRequestID()

		if len(id) != 32 {
			t.Fatalf("request id %q has length %d, want 32", id, len(id))
		}

		for _, char := range id {
			if !strings.ContainsRune("0123456789abcdef", char) {
				t.Fatalf("request id %q is not lowercase hex", id)
			}
		}

		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate request id %q", id)
		}

		seen[id] = struct{}{}
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	run := func(t *testing.T, incoming string) string {
		t.Helper()

		var fromContext string

		handler := requestIDMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			fromContext = requestIDFromContext(r.Context())
		}))

		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		if incoming != "" {
			request.Header.Set("X-Request-Id", incoming)
		}

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if got := recorder.Header().Get("X-Request-Id"); got != fromContext {
			t.Fatalf("header %q does not match context value %q", got, fromContext)
		}

		return fromContext
	}

	t.Run("incoming is trimmed and kept", func(t *testing.T) {
		if got := run(t, "  trace-123  "); got != "trace-123" {
			t.Fatalf("request id = %q, want trace-123", got)
		}
	})

	t.Run("blank is generated", func(t *testing.T) {
		if got := run(t, ""); len(got) != 32 {
			t.Fatalf("request id = %q, want 32 characters", got)
		}
	})

	t.Run("over-long is replaced", func(t *testing.T) {
		incoming := strings.Repeat("a", 129)
		if got := run(t, incoming); got == incoming {
			t.Fatal("over-long request id was not replaced")
		}
	})

	t.Run("at the limit is kept", func(t *testing.T) {
		incoming := strings.Repeat("a", 128)
		if got := run(t, incoming); got != incoming {
			t.Fatalf("request id = %q, want the incoming value", got)
		}
	})
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{name: "absent"},
		{name: "scheme without separator", header: "bearer"},
		{name: "prefix only", header: "bearer "},
		{name: "other scheme", header: "Basic dXNlcjpwYXNz"},
		{name: "foreign scheme with token", header: "Token abc123"},
		{name: "tab separator", header: "bearer\tabc123"},
		{name: "canonical", header: "Bearer abc123", want: "abc123"},
		{name: "lowercase", header: "bearer abc123", want: "abc123"},
		{name: "padded", header: "BEARER   abc123   ", want: "abc123"},
		{name: "mixed case", header: "BeArEr abc123", want: "abc123"},
		{name: "empty token", header: "Bearer    "},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
			if test.header != "" {
				request.Header.Set("Authorization", test.header)
			}

			if got := bearerToken(request); got != test.want {
				t.Fatalf("bearerToken(%q) = %q, want %q", test.header, got, test.want)
			}
		})
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("plain request", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		headers := recorder.Header()

		if got := headers.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("X-Content-Type-Options = %q", got)
		}

		if got := headers.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("X-Frame-Options = %q", got)
		}

		if got := headers.Get("Content-Security-Policy"); got != contentSecurityPolicy {
			t.Errorf("Content-Security-Policy = %q", got)
		}

		if got := headers.Get("Strict-Transport-Security"); got != "" {
			t.Errorf("Strict-Transport-Security = %q, want it absent", got)
		}
	})

	t.Run("secure request", func(t *testing.T) {
		request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		request.TLS = &tls.ConnectionState{Version: tls.VersionTLS13}
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if got := recorder.Header().Get("Strict-Transport-Security"); !strings.Contains(got, "max-age=") {
			t.Fatalf("Strict-Transport-Security = %q", got)
		}
	})
}

func TestAccessLogMiddlewareRecordsStatus(t *testing.T) {
	var buffer bytes.Buffer

	logger := slog.New(slog.NewJSONHandler(&buffer, nil))
	handler := accessLogMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
	}))

	handler.ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/anekdots", nil),
	)

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buffer.Bytes()), &entry); err != nil {
		t.Fatalf("decoding log entry: %v", err)
	}

	if got := entry["status"]; got != float64(http.StatusOK) {
		t.Fatalf("logged status = %v, want 200", got)
	}

	if got := entry["bytes"]; got != float64(len("hello")) {
		t.Fatalf("logged bytes = %v, want %d", got, len("hello"))
	}

	if got := entry["path"]; got != "/api/v1/anekdots" {
		t.Fatalf("logged path = %v", got)
	}
}

func TestCollectionAnekdotIDs(t *testing.T) {
	handler := &CollectionHandler{}

	tests := []struct {
		name       string
		collection string
		anekdot    string
		wantIDs    [2]int64
		ok         bool
	}{
		{name: "valid", collection: "5", anekdot: "9", wantIDs: [2]int64{5, 9}, ok: true},
		{name: "bad collection", collection: "abc", anekdot: "9"},
		{name: "bad anekdot", collection: "5", anekdot: "abc"},
		{name: "zero collection", collection: "0", anekdot: "9"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/", nil)
			request.SetPathValue("collection_id", test.collection)
			request.SetPathValue("anekdot_id", test.anekdot)
			recorder := httptest.NewRecorder()

			collectionID, anekdotID, ok := handler.collectionAnekdotIDs(recorder, request)

			if ok != test.ok {
				t.Fatalf("ok = %v, want %v", ok, test.ok)
			}

			if got := [2]int64{collectionID, anekdotID}; got != test.wantIDs {
				t.Fatalf("ids = %v, want %v", got, test.wantIDs)
			}

			if ok && recorder.Body.Len() != 0 {
				t.Fatalf("valid ids wrote a body: %s", recorder.Body.String())
			}

			if !ok && recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", recorder.Code)
			}
		})
	}
}

func TestIPRateLimiterEvictsOnlyExpiredEntries(t *testing.T) {
	limiter := newIPRateLimiter(rate.Inf, 1)
	limiter.maxEntries = 2
	limiter.entryTTL = time.Minute

	now := time.Now()
	entry := func(lastSeen time.Time) *ipLimiterEntry {
		return &ipLimiterEntry{limiter: rate.NewLimiter(rate.Inf, 1), lastSeen: lastSeen}
	}

	limiter.entries["expired"] = entry(now.Add(-2 * time.Minute))
	limiter.entries["boundary"] = entry(now.Add(-time.Minute))
	limiter.entries["fresh"] = entry(now)

	limiter.evictStale(now)

	if _, ok := limiter.entries["expired"]; ok {
		t.Fatal("entry older than the TTL was kept")
	}

	if _, ok := limiter.entries["boundary"]; !ok {
		t.Fatal("entry at exactly the TTL was evicted")
	}

	if _, ok := limiter.entries["fresh"]; !ok {
		t.Fatal("fresh entry was evicted")
	}
}
