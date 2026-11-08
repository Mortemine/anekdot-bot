package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexey/anekdot-bot/backend/internal/config"
	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

const testAdminToken = "0123456789abcdef"

type categoryStub struct {
	categories []domain.Category
	listErr    error
	byID       map[int]domain.Category
	getErr     error
}

func (s *categoryStub) List(context.Context) ([]domain.Category, error) {
	return s.categories, s.listErr
}

func (s *categoryStub) Get(_ context.Context, id int) (domain.Category, error) {
	if s.getErr != nil {
		return domain.Category{}, s.getErr
	}

	category, ok := s.byID[id]
	if !ok {
		return domain.Category{}, domain.ErrNotFound
	}

	return category, nil
}

type anekdotStub struct {
	random             domain.Anekdot
	randomErr          error
	byCategory         domain.Anekdot
	byCategoryErr      error
	counts             map[int]int
	countsErr          error
	receivedCategoryID int
	byCategoryCalled   bool
}

func (s *anekdotStub) Random(context.Context) (domain.Anekdot, error) {
	return s.random, s.randomErr
}

func (s *anekdotStub) RandomByCategory(_ context.Context, categoryID int) (domain.Anekdot, error) {
	s.byCategoryCalled = true
	s.receivedCategoryID = categoryID

	return s.byCategory, s.byCategoryErr
}

func (s *anekdotStub) CountsByCategory(context.Context) (map[int]int, error) {
	return s.counts, s.countsErr
}

type externalStub struct {
	text string
	err  error
}

func (s *externalStub) Random(context.Context) (string, error) {
	return s.text, s.err
}

type feedbackStub struct {
	created     domain.Feedback
	createErr   error
	messages    []domain.Feedback
	listErr     error
	lastMessage string
	lastContact *string
	lastLimit   int
	lastOffset  int
}

func (s *feedbackStub) Create(_ context.Context, message string, contact *string) (domain.Feedback, error) {
	s.lastMessage = message
	s.lastContact = contact

	return s.created, s.createErr
}

func (s *feedbackStub) List(_ context.Context, limit, offset int) ([]domain.Feedback, error) {
	s.lastLimit = limit
	s.lastOffset = offset

	return s.messages, s.listErr
}

type pingerStub struct {
	err error
}

func (s pingerStub) Ping(context.Context) error {
	return s.err
}

type fixture struct {
	categories *categoryStub
	anekdots   *anekdotStub
	external   *externalStub
	feedback   *feedbackStub
	pinger     pingerStub
	cfg        *config.Config
}

func newFixture() *fixture {
	return &fixture{
		categories: &categoryStub{byID: map[int]domain.Category{
			0: {ID: 0, Name: "Вне категорий"},
			1: {ID: 1, Name: "Разные"},
		}},
		anekdots: &anekdotStub{
			random:     domain.Anekdot{ID: 1, Text: "глобальный"},
			byCategory: domain.Anekdot{ID: 2, CategoryID: 1, Text: "категорийный"},
			counts:     map[int]int{},
		},
		external: &externalStub{text: "внешний"},
		feedback: &feedbackStub{
			created: domain.Feedback{
				ID:        1,
				Message:   "привет",
				Status:    domain.FeedbackStatusNew,
				CreatedAt: time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
			},
		},
		pinger: pingerStub{},
		cfg: &config.Config{
			HTTPAddr:    ":8080",
			AdminToken:  testAdminToken,
			CORSOrigins: []string{"http://localhost:5173"},
		},
	}
}

func (f *fixture) handler(t *testing.T) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler, err := NewRouter(
		f.cfg,
		logger,
		usecase.NewAnekdotService(f.categories, f.anekdots, f.external, logger),
		usecase.NewFeedbackService(f.feedback),
		f.pinger,
	)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	return handler
}

func perform(
	t *testing.T,
	handler http.Handler,
	method, target string,
	body io.Reader,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), method, target, body)
	for key, value := range headers {
		request.Header.Set(key, value)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func decodeBody[T any](t *testing.T, recorder *httptest.ResponseRecorder) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decoding response body %q: %v", recorder.Body.String(), err)
	}

	return value
}

func TestCategoriesEndpoint(t *testing.T) {
	f := newFixture()
	f.categories.categories = []domain.Category{{ID: 1, Name: "Разные"}, {ID: 2, Name: "Школьные"}}
	f.anekdots.counts = map[int]int{1: 3}

	recorder := perform(
		t,
		f.handler(t),
		http.MethodGet,
		"/api/v1/categories",
		nil,
		map[string]string{"X-Request-Id": "test-request-id"},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("X-Request-Id"); got != "test-request-id" {
		t.Errorf("X-Request-Id = %q", got)
	}
	if got := recorder.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if recorder.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy header is missing")
	}

	payload := decodeBody[categoriesResponse](t, recorder)
	if len(payload.Categories) != 2 {
		t.Fatalf("len(categories) = %d, want 2", len(payload.Categories))
	}
	if payload.Categories[0].AnekdotCount != 3 {
		t.Errorf("count for category 1 = %d, want 3", payload.Categories[0].AnekdotCount)
	}
}

func TestRandomEndpoint(t *testing.T) {
	t.Run("global", func(t *testing.T) {
		f := newFixture()

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/anekdots/random", nil, nil)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}

		payload := decodeBody[anekdotResponse](t, recorder)
		if payload.Anekdot.Text != "глобальный" {
			t.Errorf("text = %q, want %q", payload.Anekdot.Text, "глобальный")
		}
		if f.anekdots.byCategoryCalled {
			t.Error("RandomByCategory was called for a global request")
		}
	})

	t.Run("by category", func(t *testing.T) {
		f := newFixture()

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/anekdots/random?category_id=1", nil, nil)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if f.anekdots.receivedCategoryID != 1 {
			t.Errorf("category id = %d, want 1", f.anekdots.receivedCategoryID)
		}

		payload := decodeBody[anekdotResponse](t, recorder)
		if payload.Anekdot.Text != "категорийный" {
			t.Errorf("text = %q, want %q", payload.Anekdot.Text, "категорийный")
		}
	})

	t.Run("category zero is valid", func(t *testing.T) {
		f := newFixture()

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/anekdots/random?category_id=0", nil, nil)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if !f.anekdots.byCategoryCalled || f.anekdots.receivedCategoryID != 0 {
			t.Errorf("RandomByCategory called = %v with %d, want true with 0", f.anekdots.byCategoryCalled, f.anekdots.receivedCategoryID)
		}
	})

	tests := []struct {
		name      string
		target    string
		code      int
		errorCode string
	}{
		{name: "not a number", target: "/api/v1/anekdots/random?category_id=abc", code: 400, errorCode: "invalid_input"},
		{name: "negative", target: "/api/v1/anekdots/random?category_id=-3", code: 400, errorCode: "invalid_input"},
		{name: "unknown category", target: "/api/v1/anekdots/random?category_id=99", code: 404, errorCode: "not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture()

			recorder := perform(t, f.handler(t), http.MethodGet, test.target, nil, nil)
			if recorder.Code != test.code {
				t.Fatalf("status = %d, want %d", recorder.Code, test.code)
			}

			payload := decodeBody[errorEnvelope](t, recorder)
			if payload.Error.Code != test.errorCode {
				t.Errorf("error code = %q, want %q", payload.Error.Code, test.errorCode)
			}
		})
	}
}

func TestExternalEndpoint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture()

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/anekdots/external", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}

		payload := decodeBody[externalAnekdotResponse](t, recorder)
		if payload.Anekdot.Text != "внешний" || payload.Anekdot.Source != externalAnekdotSource {
			t.Errorf("payload = %+v", payload.Anekdot)
		}
	})

	t.Run("upstream failure", func(t *testing.T) {
		f := newFixture()
		f.external.err = domain.ErrUpstreamUnavailable

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/anekdots/external", nil, nil)
		if recorder.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502", recorder.Code)
		}

		payload := decodeBody[errorEnvelope](t, recorder)
		if payload.Error.Code != "upstream_unavailable" {
			t.Errorf("error code = %q", payload.Error.Code)
		}
	})
}

func TestFeedbackCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture()

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/feedback",
			strings.NewReader(`{"message":"привет","contact":"@user"}`),
			map[string]string{"Content-Type": "application/json"},
		)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
		}
		if f.feedback.lastMessage != "привет" {
			t.Errorf("stored message = %q", f.feedback.lastMessage)
		}
		if f.feedback.lastContact == nil || *f.feedback.lastContact != "@user" {
			t.Errorf("stored contact = %v", f.feedback.lastContact)
		}

		payload := decodeBody[feedbackResponse](t, recorder)
		if payload.Feedback.Status != string(domain.FeedbackStatusNew) {
			t.Errorf("status = %q", payload.Feedback.Status)
		}
	})

	tests := []struct {
		name      string
		body      string
		code      int
		errorCode string
	}{
		{
			name:      "empty message",
			body:      `{"message":"   "}`,
			code:      400,
			errorCode: "invalid_input",
		},
		{
			name:      "unknown field",
			body:      `{"message":"привет","admin":true}`,
			code:      400,
			errorCode: "invalid_json",
		},
		{
			name:      "malformed json",
			body:      `{"message":`,
			code:      400,
			errorCode: "invalid_json",
		},
		{
			name:      "trailing content",
			body:      `{"message":"привет"}{"message":"ещё"}`,
			code:      400,
			errorCode: "invalid_json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture()

			recorder := perform(
				t,
				f.handler(t),
				http.MethodPost,
				"/api/v1/feedback",
				strings.NewReader(test.body),
				map[string]string{"Content-Type": "application/json"},
			)
			if recorder.Code != test.code {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, test.code, recorder.Body.String())
			}

			payload := decodeBody[errorEnvelope](t, recorder)
			if payload.Error.Code != test.errorCode {
				t.Errorf("error code = %q, want %q", payload.Error.Code, test.errorCode)
			}
		})
	}

	t.Run("payload too large", func(t *testing.T) {
		f := newFixture()
		body := `{"message":"` + strings.Repeat("а", maxFeedbackBodyBytes) + `"}`

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/feedback",
			strings.NewReader(body),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", recorder.Code)
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		f := newFixture()
		f.feedback.createErr = errors.New("boom")

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/feedback",
			strings.NewReader(`{"message":"привет"}`),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", recorder.Code)
		}

		payload := decodeBody[errorEnvelope](t, recorder)
		if payload.Error.Code != "internal_error" {
			t.Errorf("error code = %q", payload.Error.Code)
		}
	})
}

func TestFeedbackRateLimit(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	for attempt := 1; attempt <= feedbackRateBurst; attempt++ {
		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/feedback",
			strings.NewReader(`{"message":"привет"}`),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("attempt %d: status = %d, want 201", attempt, recorder.Code)
		}
	}

	recorder := perform(
		t,
		handler,
		http.MethodPost,
		"/api/v1/feedback",
		strings.NewReader(`{"message":"привет"}`),
		map[string]string{"Content-Type": "application/json"},
	)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", recorder.Header().Get("Retry-After"))
	}
}

func TestAdminFeedback(t *testing.T) {
	f := newFixture()
	f.feedback.messages = []domain.Feedback{{
		ID:        5,
		Message:   "сообщение",
		Status:    domain.FeedbackStatusNew,
		CreatedAt: time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	}}
	handler := f.handler(t)

	t.Run("missing token", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/admin/feedback", nil, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}
		if recorder.Header().Get("WWW-Authenticate") == "" {
			t.Error("WWW-Authenticate header is missing")
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		recorder := perform(
			t,
			handler,
			http.MethodGet,
			"/api/v1/admin/feedback",
			nil,
			map[string]string{"X-Admin-Token": "wrong-token"},
		)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}
	})

	t.Run("header token", func(t *testing.T) {
		recorder := perform(
			t,
			handler,
			http.MethodGet,
			"/api/v1/admin/feedback?limit=500&offset=-1",
			nil,
			map[string]string{"X-Admin-Token": testAdminToken},
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[feedbackListResponse](t, recorder)
		if len(payload.Feedback) != 1 || payload.Feedback[0].Message != "сообщение" {
			t.Errorf("payload = %+v", payload.Feedback)
		}
		if payload.Pagination.Limit != 100 || payload.Pagination.Offset != 0 {
			t.Errorf("pagination = %+v, want limit 100 offset 0", payload.Pagination)
		}
		if f.feedback.lastLimit != 100 || f.feedback.lastOffset != 0 {
			t.Errorf("service received limit=%d offset=%d", f.feedback.lastLimit, f.feedback.lastOffset)
		}
	})

	t.Run("bearer token", func(t *testing.T) {
		recorder := perform(
			t,
			handler,
			http.MethodGet,
			"/api/v1/admin/feedback",
			nil,
			map[string]string{"Authorization": "Bearer " + testAdminToken},
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
	})
}

func TestAdminDisabled(t *testing.T) {
	f := newFixture()
	f.cfg.AdminToken = ""

	recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/admin/feedback", nil, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestHealthEndpoint(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		f := newFixture()

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/health", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}

		payload := decodeBody[healthResponse](t, recorder)
		if payload.Status != "ok" || payload.Database != "ok" {
			t.Errorf("payload = %+v", payload)
		}
	})

	t.Run("degraded", func(t *testing.T) {
		f := newFixture()
		f.pinger.err = errors.New("connection refused")

		recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/health", nil, nil)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", recorder.Code)
		}

		payload := decodeBody[healthResponse](t, recorder)
		if payload.Status != "degraded" {
			t.Errorf("status = %q", payload.Status)
		}
	})
}

func TestUnknownAPIEndpoint(t *testing.T) {
	f := newFixture()

	recorder := perform(t, f.handler(t), http.MethodGet, "/api/v1/unknown", nil, nil)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}

	payload := decodeBody[errorEnvelope](t, recorder)
	if payload.Error.Code != "not_found" {
		t.Errorf("error code = %q", payload.Error.Code)
	}
}

func TestCORS(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	t.Run("allowed preflight", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodOptions, "/api/v1/categories", nil, map[string]string{
			"Origin":                        "http://localhost:5173",
			"Access-Control-Request-Method": "GET",
		})
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", recorder.Code)
		}
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("Access-Control-Allow-Origin = %q", got)
		}
	})

	t.Run("allowed request", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/categories", nil, map[string]string{
			"Origin": "http://localhost:5173",
		})
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
			t.Errorf("Access-Control-Allow-Origin = %q", got)
		}
	})

	t.Run("disallowed origin", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/categories", nil, map[string]string{
			"Origin": "https://evil.example.com",
		})
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
			t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
		}
		if recorder.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", recorder.Code)
		}
	})
}

func TestSPAFallback(t *testing.T) {
	directory := t.TempDir()
	writeFile(t, filepath.Join(directory, "index.html"), "<html>spa-index</html>")
	writeFile(t, filepath.Join(directory, "assets", "app.js"), "console.log(1)")

	f := newFixture()
	f.cfg.WebDir = directory
	handler := f.handler(t)

	t.Run("deep route serves index", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/some/deep/route", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if !strings.Contains(recorder.Body.String(), "spa-index") {
			t.Errorf("body = %q", recorder.Body.String())
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("Cache-Control = %q", got)
		}
	})

	t.Run("hashed asset is cached", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/assets/app.js", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
			t.Errorf("Cache-Control = %q", got)
		}
	})

	t.Run("api routes stay json", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/unknown", nil, nil)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
		if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
			t.Errorf("Content-Type = %q", recorder.Header().Get("Content-Type"))
		}
	})
}

func TestRecovererMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := chain(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			panic("boom")
		}),
		requestIDMiddleware,
		accessLogMiddleware(logger),
		recovererMiddleware(logger),
	)

	recorder := perform(t, handler, http.MethodGet, "/panic", nil, nil)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
}

func TestParseBoundedInt(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fallback int
		minimum  int
		maximum  int
		want     int
	}{
		{name: "empty", raw: "", fallback: 50, minimum: 1, maximum: 100, want: 50},
		{name: "invalid", raw: "abc", fallback: 50, minimum: 1, maximum: 100, want: 50},
		{name: "below", raw: "-5", fallback: 50, minimum: 0, maximum: 100, want: 0},
		{name: "above", raw: "500", fallback: 50, minimum: 1, maximum: 100, want: 100},
		{name: "valid", raw: "25", fallback: 50, minimum: 1, maximum: 100, want: 25},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseBoundedInt(test.raw, test.fallback, test.minimum, test.maximum); got != test.want {
				t.Errorf("parseBoundedInt() = %d, want %d", got, test.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.7, 10.0.0.1")

	if got := clientIP(request, false); got != "10.0.0.5" {
		t.Errorf("clientIP(trust=false) = %q, want 10.0.0.5", got)
	}
	if got := clientIP(request, true); got != "203.0.113.7" {
		t.Errorf("clientIP(trust=true) = %q, want 203.0.113.7", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("creating directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
