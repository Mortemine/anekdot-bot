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

	byID        domain.Anekdot
	byIDErr     error
	created     domain.Anekdot
	createErr   error
	createArg   domain.Anekdot
	authored    []domain.Anekdot
	authoredErr error

	listed         []domain.Anekdot
	listedErr      error
	listedCategory *int
	listedLimit    int
	listedOffset   int
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

func (s *anekdotStub) ByID(context.Context, int64) (domain.Anekdot, error) {
	return s.byID, s.byIDErr
}

func (s *anekdotStub) Create(_ context.Context, anekdot domain.Anekdot) (domain.Anekdot, error) {
	s.createArg = anekdot

	return s.created, s.createErr
}

func (s *anekdotStub) ListByAuthor(context.Context, int64, int) ([]domain.Anekdot, error) {
	return s.authored, s.authoredErr
}

func (s *anekdotStub) List(_ context.Context, categoryID *int, limit, offset int) ([]domain.Anekdot, error) {
	s.listedCategory = categoryID
	s.listedLimit = limit
	s.listedOffset = offset

	return s.listed, s.listedErr
}

type externalStub struct {
	text string
	err  error
}

func (s *externalStub) Random(context.Context) (string, error) {
	return s.text, s.err
}

type userStub struct {
	byEmail   map[string]domain.Credentials
	byID      map[int64]domain.User
	nextID    int64
	createErr error
}

func (s *userStub) Create(_ context.Context, email, displayName, passwordHash string) (domain.User, error) {
	if s.createErr != nil {
		return domain.User{}, s.createErr
	}

	if _, ok := s.byEmail[email]; ok {
		return domain.User{}, domain.ErrConflict
	}

	s.nextID++

	user := domain.User{
		ID:          s.nextID,
		Email:       email,
		DisplayName: displayName,
		CreatedAt:   time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC),
	}
	s.byID[user.ID] = user
	s.byEmail[email] = domain.Credentials{User: user, PasswordHash: passwordHash}

	return user, nil
}

func (s *userStub) ByEmail(_ context.Context, email string) (domain.Credentials, error) {
	credentials, ok := s.byEmail[email]
	if !ok {
		return domain.Credentials{}, domain.ErrNotFound
	}

	return credentials, nil
}

func (s *userStub) ByID(_ context.Context, id int64) (domain.User, error) {
	user, ok := s.byID[id]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return user, nil
}

type sessionStub struct {
	users     *userStub
	sessions  map[string]domain.Session
	createErr error
	lookupErr error
	deleteErr error
	deleted   []string
}

func (s *sessionStub) Create(_ context.Context, session domain.Session) error {
	if s.createErr != nil {
		return s.createErr
	}

	s.sessions[session.TokenHash] = session

	return nil
}

func (s *sessionStub) ByTokenHash(_ context.Context, tokenHash string) (domain.User, error) {
	if s.lookupErr != nil {
		return domain.User{}, s.lookupErr
	}

	session, ok := s.sessions[tokenHash]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}

	return s.users.byID[session.UserID], nil
}

func (s *sessionStub) Delete(_ context.Context, tokenHash string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}

	delete(s.sessions, tokenHash)
	s.deleted = append(s.deleted, tokenHash)

	return nil
}

type reactionStub struct {
	list      []domain.Anekdot
	addErr    error
	removeErr error
	added     []int64
	removed   []int64
}

func (s *reactionStub) List(context.Context, int64) ([]domain.Anekdot, error) {
	return s.list, nil
}

func (s *reactionStub) Add(_ context.Context, _, anekdotID int64) error {
	if s.addErr != nil {
		return s.addErr
	}

	s.added = append(s.added, anekdotID)

	return nil
}

func (s *reactionStub) Remove(_ context.Context, _, anekdotID int64) error {
	if s.removeErr != nil {
		return s.removeErr
	}

	s.removed = append(s.removed, anekdotID)

	return nil
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
	likes      *reactionStub
	favorites  *reactionStub
	users      *userStub
	sessions   *sessionStub
	pinger     pingerStub
	cfg        *config.Config
}

const (
	testEmail    = "reader@example.com"
	testPassword = "password123"
)

func newFixture() *fixture {
	users := &userStub{
		byEmail: map[string]domain.Credentials{},
		byID:    map[int64]domain.User{},
	}

	return &fixture{
		categories: &categoryStub{byID: map[int]domain.Category{
			0: {ID: 0, Name: "Вне категорий"},
			1: {ID: 1, Name: "Разные"},
		}},
		anekdots: &anekdotStub{
			random:     domain.Anekdot{ID: 1, Text: "глобальный"},
			byCategory: domain.Anekdot{ID: 2, CategoryID: 1, Text: "категорийный"},
			counts:     map[int]int{},
			byID:       domain.Anekdot{ID: 4, CategoryID: 1, Text: "лайкнутый"},
			created:    domain.Anekdot{ID: 6, CategoryID: 1, Text: "свой анекдот"},
			authored:   []domain.Anekdot{{ID: 6, CategoryID: 1, Text: "свой анекдот"}},
			listed:     []domain.Anekdot{{ID: 3, CategoryID: 1, Text: "первый", LikeCount: 2}},
		},
		external:  &externalStub{text: "внешний"},
		likes:     &reactionStub{list: []domain.Anekdot{{ID: 4, CategoryID: 1, Text: "лайкнутый", LikeCount: 5}}},
		favorites: &reactionStub{list: []domain.Anekdot{{ID: 7, CategoryID: 1, Text: "избранный", LikeCount: 3}}},
		users:     users,
		sessions:  &sessionStub{users: users, sessions: map[string]domain.Session{}},
		pinger:    pingerStub{},
		cfg: &config.Config{
			HTTPAddr:    ":8080",
			SessionTTL:  time.Hour,
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
		usecase.NewAnekdotService(f.categories, f.anekdots, f.likes, f.favorites, f.external, logger),
		usecase.NewAuthService(f.users, f.sessions, f.cfg.SessionTTL, logger),
		f.pinger,
	)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}

	return handler
}

func (f *fixture) token(t *testing.T, handler http.Handler) string {
	t.Helper()

	recorder := perform(
		t,
		handler,
		http.MethodPost,
		"/api/v1/auth/register",
		strings.NewReader(`{"email":"`+testEmail+`","password":"`+testPassword+`"}`),
		map[string]string{"Content-Type": "application/json"},
	)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201: %s", recorder.Code, recorder.Body.String())
	}

	return decodeBody[authResponse](t, recorder).Token
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
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

func TestAuthRegister(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture()

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/auth/register",
			strings.NewReader(`{"email":"New@Example.com","password":"password123","display_name":"Читатель"}`),
			map[string]string{"Content-Type": "application/json"},
		)

		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[authResponse](t, recorder)
		if payload.Token == "" {
			t.Error("token is empty")
		}
		if payload.User.Email != "new@example.com" {
			t.Errorf("email = %q, want normalized", payload.User.Email)
		}
		if payload.User.DisplayName != "Читатель" {
			t.Errorf("display name = %q", payload.User.DisplayName)
		}
	})

	t.Run("display name falls back to email", func(t *testing.T) {
		f := newFixture()

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/auth/register",
			strings.NewReader(`{"email":"reader@example.com","password":"password123"}`),
			map[string]string{"Content-Type": "application/json"},
		)

		payload := decodeBody[authResponse](t, recorder)
		if payload.User.DisplayName != "reader" {
			t.Errorf("display name = %q, want reader", payload.User.DisplayName)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		f := newFixture()
		handler := f.handler(t)
		f.token(t, handler)

		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/auth/register",
			strings.NewReader(`{"email":"`+testEmail+`","password":"password123"}`),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", recorder.Code, recorder.Body.String())
		}
	})

	tests := []struct {
		name      string
		body      string
		code      int
		errorCode string
	}{
		{
			name:      "short password",
			body:      `{"email":"reader@example.com","password":"short"}`,
			code:      400,
			errorCode: "invalid_input",
		},
		{
			name:      "invalid email",
			body:      `{"email":"не почта","password":"password123"}`,
			code:      400,
			errorCode: "invalid_input",
		},
		{
			name:      "unknown field",
			body:      `{"email":"reader@example.com","password":"password123","admin":true}`,
			code:      400,
			errorCode: "invalid_json",
		},
		{
			name:      "malformed json",
			body:      `{"email":`,
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
				"/api/v1/auth/register",
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
}

func TestAuthLogin(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		f := newFixture()
		handler := f.handler(t)
		f.token(t, handler)

		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/auth/login",
			strings.NewReader(`{"email":"`+testEmail+`","password":"`+testPassword+`"}`),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		if decodeBody[authResponse](t, recorder).Token == "" {
			t.Error("token is empty")
		}
	})

	tests := []struct {
		name string
		body string
	}{
		{
			name: "wrong password",
			body: `{"email":"` + testEmail + `","password":"wrong-password"}`,
		},
		{
			name: "unknown email",
			body: `{"email":"nobody@example.com","password":"` + testPassword + `"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture()
			handler := f.handler(t)
			f.token(t, handler)

			recorder := perform(
				t,
				handler,
				http.MethodPost,
				"/api/v1/auth/login",
				strings.NewReader(test.body),
				map[string]string{"Content-Type": "application/json"},
			)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", recorder.Code, recorder.Body.String())
			}
			if recorder.Header().Get("WWW-Authenticate") == "" {
				t.Error("WWW-Authenticate header is missing")
			}

			payload := decodeBody[errorEnvelope](t, recorder)
			if payload.Error.Code != "unauthorized" {
				t.Errorf("error code = %q, want unauthorized", payload.Error.Code)
			}
		})
	}
}

func TestAuthMeAndLogout(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	t.Run("me requires a token", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/auth/me", nil, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/auth/me", nil, bearer("unknown-token"))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", recorder.Code)
		}
	})

	token := f.token(t, handler)

	t.Run("me", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/auth/me", nil, bearer(token))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[sessionResponse](t, recorder)
		if payload.User.Email != testEmail {
			t.Errorf("email = %q, want %q", payload.User.Email, testEmail)
		}
	})

	t.Run("logout", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodPost, "/api/v1/auth/logout", nil, bearer(token))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
		}

		recorder = perform(t, handler, http.MethodGet, "/api/v1/auth/me", nil, bearer(token))
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status after logout = %d, want 401", recorder.Code)
		}
	})
}

func TestLikesRequireToken(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	cases := []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/api/v1/likes"},
		{method: http.MethodPut, target: "/api/v1/likes/4"},
		{method: http.MethodDelete, target: "/api/v1/likes/4"},
	}

	for _, test := range cases {
		recorder := perform(t, handler, test.method, test.target, nil, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", test.method, test.target, recorder.Code)
		}
	}
}

func TestLikesEndpoints(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	token := f.token(t, handler)

	t.Run("list", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/likes", nil, bearer(token))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[anekdotsResponse](t, recorder)
		if len(payload.Anekdots) != 1 || payload.Anekdots[0].ID != 4 {
			t.Errorf("likes = %+v", payload.Anekdots)
		}
		if payload.Anekdots[0].LikeCount != 5 {
			t.Errorf("like_count = %d, want 5", payload.Anekdots[0].LikeCount)
		}
	})

	t.Run("add", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodPut, "/api/v1/likes/4", nil, bearer(token))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
		}
		if len(f.likes.added) != 1 || f.likes.added[0] != 4 {
			t.Errorf("added = %v, want [4]", f.likes.added)
		}
	})

	t.Run("remove", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodDelete, "/api/v1/likes/4", nil, bearer(token))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
		}
		if len(f.likes.removed) != 1 || f.likes.removed[0] != 4 {
			t.Errorf("removed = %v, want [4]", f.likes.removed)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		for _, target := range []string{"/api/v1/likes/0", "/api/v1/likes/abc"} {
			recorder := perform(t, handler, http.MethodPut, target, nil, bearer(token))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("PUT %s status = %d, want 400", target, recorder.Code)
			}
		}
	})

	t.Run("missing anecdote", func(t *testing.T) {
		f.anekdots.byIDErr = domain.ErrNotFound
		t.Cleanup(func() { f.anekdots.byIDErr = nil })

		recorder := perform(t, handler, http.MethodPut, "/api/v1/likes/99", nil, bearer(token))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
	})
}

func TestFavoritesRequireToken(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	cases := []struct {
		method string
		target string
	}{
		{method: http.MethodGet, target: "/api/v1/favorites"},
		{method: http.MethodPut, target: "/api/v1/favorites/4"},
		{method: http.MethodDelete, target: "/api/v1/favorites/4"},
	}

	for _, test := range cases {
		recorder := perform(t, handler, test.method, test.target, nil, nil)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401", test.method, test.target, recorder.Code)
		}
	}
}

func TestFavoritesEndpoints(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	token := f.token(t, handler)

	t.Run("list", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/favorites", nil, bearer(token))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[anekdotsResponse](t, recorder)
		if len(payload.Anekdots) != 1 || payload.Anekdots[0].ID != 7 {
			t.Errorf("favorites = %+v", payload.Anekdots)
		}
	})

	t.Run("add", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodPut, "/api/v1/favorites/4", nil, bearer(token))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
		}

		if len(f.favorites.added) != 1 || f.favorites.added[0] != 4 {
			t.Errorf("added = %v, want [4]", f.favorites.added)
		}

		if len(f.likes.added) != 0 {
			t.Errorf("likes added = %v, want none", f.likes.added)
		}
	})

	t.Run("remove", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodDelete, "/api/v1/favorites/4", nil, bearer(token))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", recorder.Code, recorder.Body.String())
		}

		if len(f.favorites.removed) != 1 || f.favorites.removed[0] != 4 {
			t.Errorf("removed = %v, want [4]", f.favorites.removed)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		for _, target := range []string{"/api/v1/favorites/0", "/api/v1/favorites/abc"} {
			recorder := perform(t, handler, http.MethodPut, target, nil, bearer(token))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("PUT %s status = %d, want 400", target, recorder.Code)
			}
		}
	})

	t.Run("missing anecdote", func(t *testing.T) {
		f.anekdots.byIDErr = domain.ErrNotFound
		t.Cleanup(func() { f.anekdots.byIDErr = nil })

		recorder := perform(t, handler, http.MethodPut, "/api/v1/favorites/99", nil, bearer(token))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
	})
}

func TestAnekdotsEndpoint(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	t.Run("lists a category page", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/anekdots?category_id=1&limit=5&offset=10", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}

		payload := decodeBody[anekdotsResponse](t, recorder)
		if len(payload.Anekdots) != 1 || payload.Anekdots[0].LikeCount != 2 {
			t.Errorf("anekdots = %+v", payload.Anekdots)
		}
		if f.anekdots.listedCategory == nil || *f.anekdots.listedCategory != 1 {
			t.Errorf("listed category = %v, want 1", f.anekdots.listedCategory)
		}
		if f.anekdots.listedLimit != 5 || f.anekdots.listedOffset != 10 {
			t.Errorf("limit = %d, offset = %d, want 5 and 10", f.anekdots.listedLimit, f.anekdots.listedOffset)
		}
	})

	t.Run("defaults paging", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/anekdots", nil, nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
		}
		if f.anekdots.listedCategory != nil {
			t.Errorf("listed category = %v, want nil", f.anekdots.listedCategory)
		}
	})

	t.Run("invalid paging", func(t *testing.T) {
		for _, target := range []string{"/api/v1/anekdots?limit=abc", "/api/v1/anekdots?offset=-1"} {
			recorder := perform(t, handler, http.MethodGet, target, nil, nil)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("GET %s status = %d, want 400", target, recorder.Code)
			}
		}
	})

	t.Run("unknown category", func(t *testing.T) {
		recorder := perform(t, handler, http.MethodGet, "/api/v1/anekdots?category_id=42", nil, nil)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", recorder.Code)
		}
	})
}

func TestSubmitAnekdot(t *testing.T) {
	t.Run("requires a token", func(t *testing.T) {
		f := newFixture()

		recorder := perform(
			t,
			f.handler(t),
			http.MethodPost,
			"/api/v1/anekdots",
			strings.NewReader(`{"category_id":1,"text":"свой анекдот"}`),
			map[string]string{"Content-Type": "application/json"},
		)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("success", func(t *testing.T) {
		f := newFixture()
		handler := f.handler(t)
		token := f.token(t, handler)

		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/anekdots",
			strings.NewReader(`{"category_id":1,"text":"свой анекдот"}`),
			bearer(token),
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", recorder.Code, recorder.Body.String())
		}

		if f.anekdots.createArg.AuthorID == nil {
			t.Error("author is not set on the stored anecdote")
		}

		payload := decodeBody[anekdotResponse](t, recorder)
		if payload.Anekdot.ID != 6 {
			t.Errorf("id = %d, want 6", payload.Anekdot.ID)
		}
	})

	tests := []struct {
		name      string
		body      string
		code      int
		errorCode string
	}{
		{
			name:      "short text",
			body:      `{"category_id":1,"text":"мало"}`,
			code:      400,
			errorCode: "invalid_input",
		},
		{
			name:      "negative category",
			body:      `{"category_id":-1,"text":"нормальный анекдот"}`,
			code:      400,
			errorCode: "invalid_input",
		},
		{
			name:      "unknown category",
			body:      `{"category_id":77,"text":"нормальный анекдот"}`,
			code:      404,
			errorCode: "not_found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newFixture()
			handler := f.handler(t)
			token := f.token(t, handler)

			recorder := perform(
				t,
				handler,
				http.MethodPost,
				"/api/v1/anekdots",
				strings.NewReader(test.body),
				bearer(token),
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

	t.Run("duplicate text", func(t *testing.T) {
		f := newFixture()
		f.anekdots.createErr = domain.ErrConflict
		handler := f.handler(t)
		token := f.token(t, handler)

		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/anekdots",
			strings.NewReader(`{"category_id":1,"text":"повторяющийся анекдот"}`),
			bearer(token),
		)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409: %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("payload too large", func(t *testing.T) {
		f := newFixture()
		handler := f.handler(t)
		token := f.token(t, handler)
		body := `{"category_id":1,"text":"` + strings.Repeat("а", maxAnekdotBodyBytes) + `"}`

		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/anekdots",
			strings.NewReader(body),
			bearer(token),
		)
		if recorder.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d, want 413", recorder.Code)
		}
	})
}

func TestMineEndpoint(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)

	if recorder := perform(t, handler, http.MethodGet, "/api/v1/anekdots/mine", nil, nil); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", recorder.Code)
	}

	token := f.token(t, handler)

	recorder := perform(t, handler, http.MethodGet, "/api/v1/anekdots/mine", nil, bearer(token))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	payload := decodeBody[anekdotsResponse](t, recorder)
	if len(payload.Anekdots) != 1 || payload.Anekdots[0].ID != 6 {
		t.Errorf("anecdotes = %+v", payload.Anekdots)
	}
}

func TestSubmitRateLimit(t *testing.T) {
	f := newFixture()
	handler := f.handler(t)
	token := f.token(t, handler)

	for attempt := 1; attempt <= writeRateBurst; attempt++ {
		recorder := perform(
			t,
			handler,
			http.MethodPost,
			"/api/v1/anekdots",
			strings.NewReader(`{"category_id":1,"text":"свой анекдот"}`),
			bearer(token),
		)
		if recorder.Code != http.StatusCreated {
			t.Fatalf("attempt %d: status = %d, want 201: %s", attempt, recorder.Code, recorder.Body.String())
		}
	}

	recorder := perform(
		t,
		handler,
		http.MethodPost,
		"/api/v1/anekdots",
		strings.NewReader(`{"category_id":1,"text":"свой анекдот"}`),
		bearer(token),
	)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %q, want 60", recorder.Header().Get("Retry-After"))
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
