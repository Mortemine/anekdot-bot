package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alexey/anekdotus/backend/internal/config"
	"github.com/alexey/anekdotus/backend/internal/domain"
)

const (
	defaultTestDSN         = "postgres://anekdot:anekdot@localhost:5432/anekdot?sslmode=disable"
	testCategoryBase int32 = 9000
	testHashLength         = 60
	testTokenLength        = 64
	unknownID              = 1 << 40
)

var testCategorySequence atomic.Int32

type fixtures struct {
	pool       *pgxpool.Pool
	categoryID int32
	ownerID    int64
	users      []int64
	categories []int32
}

func must(t *testing.T, err error, action string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
}

func mustNotFound(t *testing.T, err error, action string) {
	t.Helper()

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("%s: expected not found, got %v", action, err)
	}
}

func mustConflict(t *testing.T, err error, action string) {
	t.Helper()

	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("%s: expected conflict, got %v", action, err)
	}
}

func mustFail(t *testing.T, err error, action string) {
	t.Helper()

	if err == nil {
		t.Fatalf("%s: expected an error", action)
	}
}

func mustEqual[A comparable](t *testing.T, got, want A, action string) {
	t.Helper()

	if got != want {
		t.Fatalf("%s: expected %v, got %v", action, want, got)
	}
}

func testDSN() string {
	if dsn := os.Getenv("TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}

	return defaultTestDSN
}

func TestOptionalID(t *testing.T) {
	tests := []struct {
		name  string
		value int64
		want  any
	}{
		{name: "unknown parent becomes NULL", value: 0, want: nil},
		{name: "negative is treated as unknown", value: -5, want: nil},
		{name: "one is kept", value: 1, want: int64(1)},
		{name: "large is kept", value: unknownID, want: int64(unknownID)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := optionalID(test.value); got != test.want {
				t.Fatalf("optionalID(%d) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func testConfig(dsn string) *config.Config {
	return &config.Config{
		DatabaseURL:       dsn,
		DBMaxConns:        4,
		DBMinConns:        0,
		DBMaxConnLifetime: time.Minute,
		DBMaxConnIdleTime: time.Minute,
	}
}

func livePool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, testDSN())
	if err == nil {
		err = pool.Ping(ctx)
	}

	if err != nil {
		if pool != nil {
			pool.Close()
		}

		t.Skipf("postgres is unavailable: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func newFixtures(t *testing.T) *fixtures {
	t.Helper()

	categoryID := testCategoryBase + testCategorySequence.Add(1)
	f := &fixtures{
		pool:       livePool(t),
		categoryID: categoryID,
		categories: []int32{categoryID},
	}

	f.exec(t, `INSERT INTO categories (id, name) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		categoryID, fmt.Sprintf("Тестовая категория %d", categoryID))

	f.exec(t, `DELETE FROM users WHERE email LIKE $1`, fmt.Sprintf("%%-%d@example.com", categoryID))

	f.ownerID = f.newUser(t, fmt.Sprintf("owner-%d@example.com", categoryID))

	t.Cleanup(func() { f.cleanup(t) })

	return f
}

func (f *fixtures) exec(t *testing.T, query string, args ...any) {
	t.Helper()

	_, err := f.pool.Exec(context.Background(), query, args...)
	must(t, err, "executing "+query)
}

func (f *fixtures) newUser(t *testing.T, email string) int64 {
	t.Helper()

	var id int64

	err := f.pool.QueryRow(
		context.Background(),
		`INSERT INTO users (email, display_name, password_hash) VALUES ($1, $2, $3)
		 ON CONFLICT (email) DO UPDATE SET display_name = EXCLUDED.display_name
		 RETURNING id`,
		email,
		"Тестовый пользователь",
		strings.Repeat("h", testHashLength),
	).Scan(&id)
	must(t, err, "creating user "+email)

	f.users = append(f.users, id)

	return id
}

func (f *fixtures) newAnekdot(t *testing.T, text string) int64 {
	t.Helper()

	var id int64

	err := f.pool.QueryRow(
		context.Background(),
		`INSERT INTO anekdots (category_id, text) VALUES ($1, $2) RETURNING id`,
		f.categoryID,
		text,
	).Scan(&id)
	must(t, err, "creating anecdote")

	return id
}

func (f *fixtures) seed(t *testing.T) domain.Anekdot {
	t.Helper()

	f.newAnekdot(t, "Первый тестовый анекдот про программиста")
	f.newAnekdot(t, "Второй тестовый анекдот про кота")

	created, err := NewAnekdotRepository(f.pool).Create(context.Background(), domain.Anekdot{
		CategoryID: int(f.categoryID),
		Text:       "Третий тестовый анекдот про слона",
		AuthorID:   &f.ownerID,
	})
	must(t, err, "creating anecdote")

	return created
}

func (f *fixtures) category() int {
	return int(f.categoryID)
}

func (f *fixtures) cleanup(t *testing.T) {
	t.Helper()

	ctx := context.Background()

	if _, err := f.pool.Exec(ctx, `DELETE FROM anekdots WHERE category_id = ANY($1)`, f.categories); err != nil {
		t.Errorf("cleaning up anecdotes: %v", err)
	}

	if _, err := f.pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, f.users); err != nil {
		t.Errorf("cleaning up users: %v", err)
	}

	if _, err := f.pool.Exec(ctx, `DELETE FROM categories WHERE id = ANY($1)`, f.categories); err != nil {
		t.Errorf("cleaning up categories: %v", err)
	}
}

func TestNewPool(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		_, err := NewPool(context.Background(), testConfig("://broken"), slog.New(slog.DiscardHandler))
		mustFail(t, err, "creating pool from a broken url")
	})

	t.Run("unreachable server", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dsn := "postgres://anekdot:anekdot@127.0.0.1:1/anekdot?sslmode=disable"

		_, err := NewPool(ctx, testConfig(dsn), slog.New(slog.DiscardHandler))
		mustFail(t, err, "creating pool for an unreachable server")
	})

	t.Run("healthy", func(t *testing.T) {
		livePool(t)

		pool, err := NewPool(context.Background(), testConfig(testDSN()), slog.New(slog.DiscardHandler))
		must(t, err, "creating pool")

		defer pool.Close()

		must(t, pool.Ping(context.Background()), "pinging pool")
	})
}

func TestCategoryRepositoryList(t *testing.T) {
	f := newFixtures(t)

	categories, err := NewCategoryRepository(f.pool).List(context.Background())
	must(t, err, "listing categories")

	sorted := true
	found := false
	previous := ""

	for _, category := range categories {
		if category.Name < previous {
			sorted = false
		}

		previous = category.Name

		found = found || category.ID == f.category()
	}

	mustEqual(t, sorted, true, "categories are sorted by name")
	mustEqual(t, found, true, "created category is listed")
}

func TestCategoryRepositoryGet(t *testing.T) {
	f := newFixtures(t)
	repo := NewCategoryRepository(f.pool)
	ctx := context.Background()

	category, err := repo.Get(ctx, f.category())
	must(t, err, "getting category")

	mustEqual(t, category.ID, f.category(), "category id")
	mustEqual(t, category.Name == "", false, "category name is present")

	missing, err := repo.Get(ctx, -1)
	mustNotFound(t, err, "getting missing category")
	mustEqual(t, missing.ID, 0, "missing category id")
}

func TestAnekdotRepositoryCreate(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)

	created, err := repo.Create(context.Background(), domain.Anekdot{
		CategoryID: f.category(),
		Text:       "Третий тестовый анекдот про слона",
		AuthorID:   &f.ownerID,
	})
	must(t, err, "creating anecdote")

	mustEqual(t, created.CategoryID, f.category(), "category id")
	mustEqual(t, created.CategoryName == "", false, "category name is present")
	mustEqual(t, created.AuthorID != nil && *created.AuthorID == f.ownerID, true, "author id")
	mustEqual(t, created.LikeCount, 0, "like count")
	mustEqual(t, created.FavoriteCount, 0, "favorite count")
}

func TestAnekdotRepositoryCreateConflict(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	text := "Повторяющийся тестовый анекдот"

	_, err := repo.Create(ctx, domain.Anekdot{CategoryID: f.category(), Text: text})
	must(t, err, "creating anecdote")

	_, err = repo.Create(ctx, domain.Anekdot{CategoryID: f.category(), Text: text})
	mustConflict(t, err, "creating a duplicate anecdote")
}

func TestAnekdotRepositoryCreateAnonymous(t *testing.T) {
	f := newFixtures(t)

	anonymous, err := NewAnekdotRepository(f.pool).Create(context.Background(), domain.Anekdot{
		CategoryID: f.category(),
		Text:       "Анекдот без автора",
	})
	must(t, err, "creating anonymous anecdote")
	mustEqual(t, anonymous.AuthorID, (*int64)(nil), "anonymous author")
}

func TestAnekdotRepositoryCreateUnknownCategory(t *testing.T) {
	f := newFixtures(t)

	_, err := NewAnekdotRepository(f.pool).Create(context.Background(), domain.Anekdot{
		CategoryID: -1,
		Text:       "Анекдот в несуществующей категории",
	})
	mustFail(t, err, "creating anecdote in an unknown category")
}

func TestAnekdotRepositoryByID(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)

	found, err := repo.ByID(ctx, created.ID)
	must(t, err, "getting anecdote by id")

	mustEqual(t, found.Text, created.Text, "anecdote text")
	mustEqual(t, found.CategoryName, created.CategoryName, "category name")

	_, err = repo.ByID(ctx, unknownID)
	mustNotFound(t, err, "getting missing anecdote")
}

func TestAnekdotRepositoryByText(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)

	found, err := repo.ByText(ctx, f.category(), created.Text)
	must(t, err, "getting anecdote by text")
	mustEqual(t, found.ID, created.ID, "anecdote id")

	_, err = repo.ByText(ctx, f.category(), "Такого анекдота нет")
	mustNotFound(t, err, "getting anecdote by unknown text")
}

func TestAnekdotRepositoryRandom(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)

	random, err := repo.Random(ctx)
	must(t, err, "getting a random anecdote")
	mustEqual(t, random.CategoryName == "", false, "random anecdote category name")

	inCategory, err := repo.RandomByCategory(ctx, f.category())
	must(t, err, "getting a random anecdote of the category")
	mustEqual(t, inCategory.CategoryID, created.CategoryID, "random anecdote category")

	_, err = repo.RandomByCategory(ctx, -1)
	mustNotFound(t, err, "getting a random anecdote of an unknown category")
}

func TestAnekdotRepositoryList(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)

	all, err := repo.List(ctx, nil, 10, 0, domain.AnekdotSortCreated)
	must(t, err, "listing anecdotes")
	mustEqual(t, containsAnekdot(all, created.ID), true, "created anecdote is listed")

	categoryID := f.category()

	local, err := repo.List(ctx, &categoryID, 10, 0, domain.AnekdotSort("bogus"))
	must(t, err, "listing anecdotes of the category")
	mustEqual(t, len(local), 3, "anecdotes in the category")

	for _, anekdot := range local {
		mustEqual(t, anekdot.CategoryID, f.category(), "anecdote category")
	}

	page, err := repo.List(ctx, &categoryID, 1, 1, domain.AnekdotSortCreated)
	must(t, err, "listing a page of anecdotes")
	mustEqual(t, len(page), 1, "anecdotes on the page")
}

func TestAnekdotRepositoryCount(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	f.seed(t)

	categoryID := f.category()

	total, err := repo.Count(ctx, nil)
	must(t, err, "counting anecdotes")
	mustEqual(t, total >= 3, true, "total anecdotes")

	local, err := repo.Count(ctx, &categoryID)
	must(t, err, "counting anecdotes of the category")
	mustEqual(t, local, 3, "anecdotes in the category")

	counts, err := repo.CountsByCategory(ctx)
	must(t, err, "counting anecdotes by category")
	mustEqual(t, counts[categoryID], 3, "anecdotes in the category")
}

func TestAnekdotRepositoryCountRejectsClosedPool(t *testing.T) {
	f := newFixtures(t)

	pool, err := pgxpool.New(context.Background(), testDSN())
	must(t, err, "creating a dedicated pool")

	repo := NewAnekdotRepository(pool)
	pool.Close()

	total, countErr := repo.Count(context.Background(), nil)
	if countErr == nil {
		t.Fatal("Count() error = nil, want an error for a closed pool")
	}

	mustEqual(t, total, 0, "count on a closed pool")
	mustEqual(t, len(f.categories), 1, "fixture categories")
}

func TestAnekdotRepositoryListByAuthor(t *testing.T) {
	f := newFixtures(t)
	created := f.seed(t)

	authored, err := NewAnekdotRepository(f.pool).ListByAuthor(context.Background(), f.ownerID, 10)
	must(t, err, "listing anecdotes of the author")
	mustEqual(t, len(authored), 1, "anecdotes of the author")
	mustEqual(t, authored[0].ID, created.ID, "author anecdote id")
}

func TestAnekdotRepositorySortByReactions(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)
	categoryID := f.category()

	must(t, NewLikeRepository(f.pool).Add(ctx, f.ownerID, created.ID), "adding like")
	must(t, NewFavoriteRepository(f.pool).Add(ctx, f.ownerID, created.ID), "adding favorite")

	reactioned, err := repo.ByID(ctx, created.ID)
	must(t, err, "getting anecdote by id")
	mustEqual(t, reactioned.LikeCount, 1, "like count")
	mustEqual(t, reactioned.FavoriteCount, 1, "favorite count")

	byLikes, err := repo.List(ctx, &categoryID, 10, 0, domain.AnekdotSortLikes)
	must(t, err, "listing anecdotes by likes")
	mustEqual(t, byLikes[0].ID, created.ID, "most liked anecdote id")

	byFavorites, err := repo.List(ctx, &categoryID, 10, 0, domain.AnekdotSortFavorites)
	must(t, err, "listing anecdotes by favorites")
	mustEqual(t, byFavorites[0].ID, created.ID, "most favorited anecdote id")
}

func TestAnekdotRepositoryPopularityByCategory(t *testing.T) {
	f := newFixtures(t)
	repo := NewAnekdotRepository(f.pool)
	ctx := context.Background()
	created := f.seed(t)

	must(t, NewLikeRepository(f.pool).Add(ctx, f.ownerID, created.ID), "adding like")
	must(t, NewFavoriteRepository(f.pool).Add(ctx, f.ownerID, created.ID), "adding favorite")

	popularity, err := repo.PopularityByCategory(ctx)
	must(t, err, "counting popularity")
	mustEqual(t, popularity[f.category()], 2, "category popularity")
}

func TestUserRepository(t *testing.T) {
	f := newFixtures(t)
	repo := NewUserRepository(f.pool)
	ctx := context.Background()
	email := fmt.Sprintf("user-%d@example.com", f.categoryID)

	user, err := repo.Create(ctx, email, "Пользователь", strings.Repeat("h", testHashLength))
	must(t, err, "creating user")
	f.users = append(f.users, user.ID)

	mustEqual(t, user.Email, email, "user email")
	mustEqual(t, user.DisplayName, "Пользователь", "user display name")
	mustEqual(t, user.CreatedAt.IsZero(), false, "user creation date")

	_, err = repo.Create(ctx, email, "Другой", strings.Repeat("h", testHashLength))
	mustConflict(t, err, "creating a duplicate user")

	uppercase, err := repo.Create(
		ctx,
		fmt.Sprintf("UPPER-%d@example.com", f.categoryID),
		"Плохой",
		strings.Repeat("h", testHashLength),
	)
	f.users = append(f.users, uppercase.ID)
	mustFail(t, err, "creating a user with an uppercase email")
}

func TestUserRepositoryByEmail(t *testing.T) {
	f := newFixtures(t)
	repo := NewUserRepository(f.pool)
	ctx := context.Background()
	email := fmt.Sprintf("user-%d@example.com", f.categoryID)

	user, err := repo.Create(ctx, email, "Пользователь", strings.Repeat("h", testHashLength))
	must(t, err, "creating user")
	f.users = append(f.users, user.ID)

	credentials, err := repo.ByEmail(ctx, email)
	must(t, err, "getting credentials")
	mustEqual(t, credentials.ID, user.ID, "credentials user id")
	mustEqual(t, credentials.PasswordHash, strings.Repeat("h", testHashLength), "password hash")

	_, err = repo.ByEmail(ctx, fmt.Sprintf("missing-%d@example.com", f.categoryID))
	mustNotFound(t, err, "getting credentials of a missing user")
}

func TestUserRepositoryByID(t *testing.T) {
	f := newFixtures(t)
	repo := NewUserRepository(f.pool)
	ctx := context.Background()
	email := fmt.Sprintf("user-%d@example.com", f.categoryID)

	user, err := repo.Create(ctx, email, "Пользователь", strings.Repeat("h", testHashLength))
	must(t, err, "creating user")
	f.users = append(f.users, user.ID)

	found, err := repo.ByID(ctx, user.ID)
	must(t, err, "getting user by id")
	mustEqual(t, found.Email, email, "user email")

	_, err = repo.ByID(ctx, unknownID)
	mustNotFound(t, err, "getting a missing user")
}

func TestSessionRepository(t *testing.T) {
	f := newFixtures(t)
	repo := NewSessionRepository(f.pool)
	ctx := context.Background()

	user, err := NewUserRepository(f.pool).Create(
		ctx,
		fmt.Sprintf("session-%d@example.com", f.categoryID),
		"Сессия",
		strings.Repeat("h", testHashLength),
	)
	must(t, err, "creating user")
	f.users = append(f.users, user.ID)

	tokenHash := strings.Repeat("a", testTokenLength)
	must(t, repo.Create(ctx, domain.Session{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Hour),
	}), "creating session")

	found, err := repo.ByTokenHash(ctx, tokenHash)
	must(t, err, "getting session")
	mustEqual(t, found.ID, user.ID, "session user id")

	must(t, repo.Delete(ctx, tokenHash), "deleting session")

	_, err = repo.ByTokenHash(ctx, tokenHash)
	mustNotFound(t, err, "getting a deleted session")
}

func TestSessionRepositoryRejectsExpired(t *testing.T) {
	f := newFixtures(t)
	repo := NewSessionRepository(f.pool)
	ctx := context.Background()

	user, err := NewUserRepository(f.pool).Create(
		ctx,
		fmt.Sprintf("expired-%d@example.com", f.categoryID),
		"Сессия",
		strings.Repeat("h", testHashLength),
	)
	must(t, err, "creating user")
	f.users = append(f.users, user.ID)

	expired := strings.Repeat("b", testTokenLength)
	must(t, repo.Create(ctx, domain.Session{
		TokenHash: expired,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(-time.Minute),
	}), "creating expired session")

	_, err = repo.ByTokenHash(ctx, expired)
	mustNotFound(t, err, "getting an expired session")

	_, err = repo.ByTokenHash(ctx, strings.Repeat("c", testTokenLength))
	mustNotFound(t, err, "getting an unknown session")

	mustFail(t, repo.Create(ctx, domain.Session{
		TokenHash: strings.Repeat("d", testTokenLength),
		UserID:    unknownID,
		ExpiresAt: time.Now().Add(time.Hour),
	}), "creating a session of an unknown user")
}

func TestFavoriteRepository(t *testing.T) {
	f := newFixtures(t)
	repo := NewFavoriteRepository(f.pool)
	ctx := context.Background()
	anekdotID := f.newAnekdot(t, "Анекдот для избранного")

	must(t, repo.Add(ctx, f.ownerID, anekdotID), "adding favorite")
	must(t, repo.Add(ctx, f.ownerID, anekdotID), "adding a duplicate favorite")
	mustNotFound(t, repo.Add(ctx, f.ownerID, unknownID), "favoriting a missing anecdote")

	favorites, err := repo.List(ctx, f.ownerID)
	must(t, err, "listing favorites")
	mustEqual(t, len(favorites), 1, "favorites")
	mustEqual(t, favorites[0].ID, anekdotID, "favorited anecdote id")
	mustEqual(t, favorites[0].FavoriteCount, 1, "favorite count")

	must(t, repo.Remove(ctx, f.ownerID, anekdotID), "removing favorite")
	must(t, repo.Remove(ctx, f.ownerID, anekdotID), "removing the missing favorite")

	empty, err := repo.List(ctx, f.ownerID)
	must(t, err, "listing favorites")
	mustEqual(t, len(empty), 0, "favorites after removal")
}

func TestLikeRepository(t *testing.T) {
	f := newFixtures(t)
	repo := NewLikeRepository(f.pool)
	ctx := context.Background()
	anekdotID := f.newAnekdot(t, "Анекдот для лайков")

	must(t, repo.Add(ctx, f.ownerID, anekdotID), "adding like")
	must(t, repo.Add(ctx, f.ownerID, anekdotID), "adding a duplicate like")
	mustNotFound(t, repo.Add(ctx, f.ownerID, unknownID), "liking a missing anecdote")

	likes, err := repo.List(ctx, f.ownerID)
	must(t, err, "listing likes")
	mustEqual(t, len(likes), 1, "likes")
	mustEqual(t, likes[0].ID, anekdotID, "liked anecdote id")
	mustEqual(t, likes[0].LikeCount, 1, "like count")

	must(t, repo.Remove(ctx, f.ownerID, anekdotID), "removing like")
	must(t, repo.Remove(ctx, f.ownerID, anekdotID), "removing the missing like")

	empty, err := repo.List(ctx, f.ownerID)
	must(t, err, "listing likes")
	mustEqual(t, len(empty), 0, "likes after removal")
}

func TestCollectionRepositoryCreate(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	mustEqual(t, collection.OwnerID, f.ownerID, "collection owner")
	mustEqual(t, collection.OwnerName == "", false, "owner name is present")
	mustEqual(t, collection.Name, "Тестовая коллекция", "collection name")
	mustEqual(t, collection.IsPublic, false, "collection visibility")
	mustEqual(t, collection.AnekdotCount, 0, "collection anecdotes")
}

func TestCollectionRepositoryCreateRejects(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	_, err = repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	mustConflict(t, err, "creating a duplicate collection")

	_, err = repo.Create(ctx, unknownID, "Чужая коллекция")
	mustNotFound(t, err, "creating a collection for a missing user")
}

func TestCollectionRepositoryByID(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	detail, err := repo.ByID(ctx, collection.ID, &f.ownerID)
	must(t, err, "getting collection")
	mustEqual(t, detail.ID, collection.ID, "collection id")

	anonymous, err := repo.ByID(ctx, collection.ID, nil)
	must(t, err, "getting collection without a viewer")
	mustEqual(t, anonymous.Favorited, false, "collection is not favorited")

	_, err = repo.ByID(ctx, collection.ID+unknownID, nil)
	mustNotFound(t, err, "getting a missing collection")
}

func TestCollectionRepositoryAnekdots(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	anekdotID := f.newAnekdot(t, "Анекдот в коллекции")

	must(t, repo.AddAnekdot(ctx, collection.ID, anekdotID), "adding anecdote")
	must(t, repo.AddAnekdot(ctx, collection.ID, anekdotID), "adding a duplicate anecdote")
	mustNotFound(t, repo.AddAnekdot(ctx, collection.ID, unknownID), "adding a missing anecdote")
	mustNotFound(t, repo.AddAnekdot(ctx, unknownID, anekdotID), "adding to a missing collection")

	anekdots, err := repo.Anekdots(ctx, collection.ID)
	must(t, err, "listing anecdotes of the collection")
	mustEqual(t, len(anekdots), 1, "collection anecdotes")
	mustEqual(t, anekdots[0].ID, anekdotID, "collection anecdote id")

	missing, err := repo.Anekdots(ctx, collection.ID+unknownID)
	must(t, err, "listing anecdotes of a missing collection")
	mustEqual(t, len(missing), 0, "anecdotes of a missing collection")

	detail, err := repo.ByID(ctx, collection.ID, &f.ownerID)
	must(t, err, "getting collection")
	mustEqual(t, detail.AnekdotCount, 1, "collection anecdote count")

	must(t, repo.RemoveAnekdot(ctx, collection.ID, anekdotID), "removing anecdote")
	mustNotFound(t, repo.RemoveAnekdot(ctx, collection.ID, anekdotID), "removing the missing anecdote")
}

func TestCollectionRepositoryUpdate(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	updated, err := repo.Update(ctx, collection.ID, f.ownerID, "Переименованная коллекция", true)
	must(t, err, "updating collection")
	mustEqual(t, updated.Name, "Переименованная коллекция", "collection name")
	mustEqual(t, updated.IsPublic, true, "collection visibility")

	_, err = repo.Update(ctx, collection.ID, unknownID, "Чужая коллекция", false)
	mustNotFound(t, err, "updating a collection of another user")
}

func TestCollectionRepositoryUpdateConflict(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	_, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	second, err := repo.Create(ctx, f.ownerID, "Вторая коллекция")
	must(t, err, "creating collection")

	_, err = repo.Update(ctx, second.ID, f.ownerID, "Тестовая коллекция", false)
	mustConflict(t, err, "renaming a collection to a taken name")
}

func TestCollectionRepositoryDelete(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	must(t, repo.Delete(ctx, collection.ID, f.ownerID), "deleting collection")
	mustNotFound(t, repo.Delete(ctx, collection.ID, f.ownerID), "deleting a deleted collection")
	mustNotFound(t, repo.Delete(ctx, collection.ID, unknownID), "deleting a collection of another user")
}

func TestCollectionRepositoryListByOwner(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	first, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	_, err = repo.Create(ctx, f.ownerID, "Вторая коллекция")
	must(t, err, "creating collection")

	anekdotID := f.newAnekdot(t, "Анекдот в коллекции")
	must(t, repo.AddAnekdot(ctx, first.ID, anekdotID), "adding anecdote")

	owned, err := repo.ListByOwner(ctx, f.ownerID, anekdotID)
	must(t, err, "listing collections of the owner")
	mustEqual(t, len(owned), 2, "collections of the owner")

	for _, item := range owned {
		mustEqual(t, item.OwnerID, f.ownerID, "collection owner")

		if item.ID == first.ID {
			mustEqual(t, item.ContainsAnekdot, true, "collection contains the anecdote")
		}
	}

	withoutAnekdot, err := repo.ListByOwner(ctx, f.ownerID, 0)
	must(t, err, "listing collections of the owner")

	for _, item := range withoutAnekdot {
		mustEqual(t, item.ContainsAnekdot, false, "collection containment flag")
	}
}

func TestCollectionRepositoryListPublic(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	published, err := repo.Update(ctx, collection.ID, f.ownerID, collection.Name, true)
	must(t, err, "publishing collection")
	mustEqual(t, published.IsPublic, true, "collection visibility")

	public, err := repo.ListPublic(ctx, f.ownerID, 0)
	must(t, err, "listing public collections")
	mustEqual(t, containsCollection(public, collection.ID), true, "public collection is listed")

	for _, item := range public {
		mustEqual(t, item.OwnerID, f.ownerID, "collection owner")
	}
}

func TestCollectionRepositoryFavorites(t *testing.T) {
	f := newFixtures(t)
	repo := NewCollectionRepository(f.pool)
	ctx := context.Background()
	viewerID := f.newUser(t, fmt.Sprintf("viewer-%d@example.com", f.categoryID))

	collection, err := repo.Create(ctx, f.ownerID, "Тестовая коллекция")
	must(t, err, "creating collection")

	must(t, repo.Favorite(ctx, viewerID, collection.ID), "favoriting collection")
	must(t, repo.Favorite(ctx, viewerID, collection.ID), "favoriting collection twice")
	mustNotFound(t, repo.Favorite(ctx, viewerID, unknownID), "favoriting a missing collection")

	favorited, err := repo.ByID(ctx, collection.ID, &viewerID)
	must(t, err, "getting collection")
	mustEqual(t, favorited.Favorited, true, "collection is favorited")

	favorites, err := repo.ListFavorites(ctx, viewerID)
	must(t, err, "listing favorite collections")
	mustEqual(t, len(favorites), 1, "favorite collections")
	mustEqual(t, favorites[0].ID, collection.ID, "favorite collection id")

	must(t, repo.Unfavorite(ctx, viewerID, collection.ID), "unfavoriting collection")
	mustNotFound(t, repo.Unfavorite(ctx, viewerID, collection.ID), "unfavoriting a missing favorite")
}

func containsAnekdot(anekdots []domain.Anekdot, id int64) bool {
	for _, anekdot := range anekdots {
		if anekdot.ID == id {
			return true
		}
	}

	return false
}

func containsCollection(collections []domain.Collection, id int64) bool {
	for _, collection := range collections {
		if collection.ID == id {
			return true
		}
	}

	return false
}
