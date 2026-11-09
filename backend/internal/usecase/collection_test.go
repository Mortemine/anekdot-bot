package usecase

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

type collectionRepositoryStub struct {
	collections   []domain.Collection
	listErr       error
	byID          domain.Collection
	byIDErr       error
	created       domain.Collection
	createErr     error
	updated       domain.Collection
	updateErr     error
	deleteErr     error
	anekdots      []domain.Anekdot
	anekdotsErr   error
	addErr        error
	removeErr     error
	favoriteErr   error
	unfavoriteErr error
	receivedName  string
	receivedID    int64
	receivedOwner int64
	receivedPub   bool
	receivedView  *int64
	receivedAnek  int64
	receivedUser  int64
}

func (s *collectionRepositoryStub) ListByOwner(
	ctx context.Context,
	ownerID, anekdotID int64,
) ([]domain.Collection, error) {
	s.receivedOwner = ownerID
	s.receivedAnek = anekdotID

	return s.collections, s.listErr
}

func (s *collectionRepositoryStub) ListPublic(
	ctx context.Context,
	viewerID, anekdotID int64,
) ([]domain.Collection, error) {
	s.receivedOwner = viewerID
	s.receivedAnek = anekdotID

	return s.collections, s.listErr
}

func (s *collectionRepositoryStub) ListFavorites(
	ctx context.Context,
	userID int64,
) ([]domain.Collection, error) {
	s.receivedUser = userID

	return s.collections, s.listErr
}

func (s *collectionRepositoryStub) ByID(
	ctx context.Context,
	id int64,
	viewerID *int64,
) (domain.Collection, error) {
	s.receivedID = id
	s.receivedView = viewerID

	if s.byIDErr != nil {
		return domain.Collection{}, s.byIDErr
	}

	return s.byID, nil
}

func (s *collectionRepositoryStub) Create(
	ctx context.Context,
	ownerID int64,
	name string,
) (domain.Collection, error) {
	s.receivedOwner = ownerID
	s.receivedName = name

	if s.createErr != nil {
		return domain.Collection{}, s.createErr
	}

	return s.created, nil
}

func (s *collectionRepositoryStub) Update(
	ctx context.Context,
	id, ownerID int64,
	name string,
	isPublic bool,
) (domain.Collection, error) {
	s.receivedID = id
	s.receivedOwner = ownerID
	s.receivedName = name
	s.receivedPub = isPublic

	if s.updateErr != nil {
		return domain.Collection{}, s.updateErr
	}

	return s.updated, nil
}

func (s *collectionRepositoryStub) Delete(ctx context.Context, id, ownerID int64) error {
	s.receivedID = id
	s.receivedOwner = ownerID

	return s.deleteErr
}

func (s *collectionRepositoryStub) AddAnekdot(ctx context.Context, collectionID, anekdotID int64) error {
	s.receivedID = collectionID
	s.receivedAnek = anekdotID

	return s.addErr
}

func (s *collectionRepositoryStub) RemoveAnekdot(ctx context.Context, collectionID, anekdotID int64) error {
	s.receivedID = collectionID
	s.receivedAnek = anekdotID

	return s.removeErr
}

func (s *collectionRepositoryStub) Anekdots(
	ctx context.Context,
	collectionID int64,
) ([]domain.Anekdot, error) {
	s.receivedID = collectionID

	return s.anekdots, s.anekdotsErr
}

func (s *collectionRepositoryStub) Favorite(ctx context.Context, userID, collectionID int64) error {
	s.receivedUser = userID
	s.receivedID = collectionID

	return s.favoriteErr
}

func (s *collectionRepositoryStub) Unfavorite(ctx context.Context, userID, collectionID int64) error {
	s.receivedUser = userID
	s.receivedID = collectionID

	return s.unfavoriteErr
}

func newCollectionService(stub *collectionRepositoryStub) *CollectionService {
	return NewCollectionService(stub, slog.New(slog.DiscardHandler))
}

func must(t *testing.T, err error, action string) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", action, err)
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

func mustInvalid(t *testing.T, err error, action string) {
	t.Helper()

	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("%s: expected invalid input, got %v", action, err)
	}
}

func mustForbidden(t *testing.T, err error, action string) {
	t.Helper()

	if !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("%s: expected forbidden, got %v", action, err)
	}
}

func mustNotFoundUsecase(t *testing.T, err error, action string) {
	t.Helper()

	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("%s: expected not found, got %v", action, err)
	}
}

func TestCollectionServiceMine(t *testing.T) {
	stub := &collectionRepositoryStub{collections: []domain.Collection{{ID: 7}}}
	service := newCollectionService(stub)

	collections, err := service.Mine(context.Background(), 3, 11)
	must(t, err, "listing own collections")
	mustEqual(t, len(collections), 1, "collections")
	mustEqual(t, stub.receivedOwner, int64(3), "owner id")
	mustEqual(t, stub.receivedAnek, int64(11), "anecdote id")

	stub.listErr = errors.New("boom")

	_, err = service.Mine(context.Background(), 3, 11)
	mustFail(t, err, "listing own collections with a repository error")
}

func TestCollectionServicePublic(t *testing.T) {
	stub := &collectionRepositoryStub{collections: []domain.Collection{{ID: 7}}}
	service := newCollectionService(stub)

	collections, err := service.Public(context.Background(), 5, 11)
	must(t, err, "listing public collections")
	mustEqual(t, len(collections), 1, "collections")
	mustEqual(t, stub.receivedOwner, int64(5), "viewer id")

	stub.listErr = errors.New("boom")

	_, err = service.Public(context.Background(), 5, 11)
	mustFail(t, err, "listing public collections with a repository error")
}

func TestCollectionServiceFavorites(t *testing.T) {
	stub := &collectionRepositoryStub{collections: []domain.Collection{{ID: 7}}}
	service := newCollectionService(stub)

	collections, err := service.Favorites(context.Background(), 5)
	must(t, err, "listing favorite collections")
	mustEqual(t, len(collections), 1, "collections")
	mustEqual(t, stub.receivedUser, int64(5), "user id")

	stub.listErr = errors.New("boom")

	_, err = service.Favorites(context.Background(), 5)
	mustFail(t, err, "listing favorite collections with a repository error")
}

func TestCollectionServiceDetail(t *testing.T) {
	stub := &collectionRepositoryStub{
		byID:     domain.Collection{ID: 7, OwnerID: 3, IsPublic: true},
		anekdots: []domain.Anekdot{{ID: 21}},
	}
	service := newCollectionService(stub)

	detail, err := service.Detail(context.Background(), 3, 7)
	must(t, err, "getting collection detail")
	mustEqual(t, detail.Collection.ID, int64(7), "collection id")
	mustEqual(t, len(detail.Anekdots), 1, "collection anecdotes")
	mustEqual(t, stub.receivedView != nil && *stub.receivedView == 3, true, "viewer pointer")

	anonymous, err := service.Detail(context.Background(), 0, 7)
	must(t, err, "getting public collection detail anonymously")
	mustEqual(t, anonymous.Collection.ID, int64(7), "collection id")
	mustEqual(t, stub.receivedView, (*int64)(nil), "anonymous viewer pointer")

	stub.anekdotsErr = errors.New("boom")

	_, err = service.Detail(context.Background(), 0, 7)
	mustFail(t, err, "getting collection detail with a repository error")
}

func TestCollectionServiceDetailHidesPrivate(t *testing.T) {
	stub := &collectionRepositoryStub{
		byID: domain.Collection{ID: 7, OwnerID: 3, IsPublic: false},
	}
	service := newCollectionService(stub)

	_, err := service.Detail(context.Background(), 4, 7)
	mustNotFoundUsecase(t, err, "getting a private collection of another user")

	detail, err := service.Detail(context.Background(), 3, 7)
	must(t, err, "getting own private collection")
	mustEqual(t, detail.Collection.ID, int64(7), "collection id")
}

func TestCollectionServiceDetailRejectsBadID(t *testing.T) {
	service := newCollectionService(&collectionRepositoryStub{})

	_, err := service.Detail(context.Background(), 3, 0)
	mustInvalid(t, err, "getting collection with a zero id")

	_, err = service.Detail(context.Background(), 3, -1)
	mustInvalid(t, err, "getting collection with a negative id")

	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 1, OwnerID: 3, IsPublic: true}}

	detail, err := newCollectionService(stub).Detail(context.Background(), 3, 1)
	must(t, err, "getting collection with the smallest valid id")
	mustEqual(t, detail.Collection.ID, int64(1), "collection id")
	mustEqual(t, stub.receivedID, int64(1), "requested id")
}

func TestCollectionServiceCreate(t *testing.T) {
	stub := &collectionRepositoryStub{created: domain.Collection{ID: 7, Name: "Коллекция"}}
	service := newCollectionService(stub)

	collection, err := service.Create(context.Background(), 3, "  Коллекция  ")
	must(t, err, "creating collection")
	mustEqual(t, collection.ID, int64(7), "collection id")
	mustEqual(t, stub.receivedName, "Коллекция", "trimmed name")
	mustEqual(t, stub.receivedOwner, int64(3), "owner id")

	stub.createErr = errors.New("boom")

	_, err = service.Create(context.Background(), 3, "Коллекция")
	mustFail(t, err, "creating collection with a repository error")
}

func TestCollectionServiceCreateRejectsBadName(t *testing.T) {
	service := newCollectionService(&collectionRepositoryStub{})

	for _, name := range []string{"", "   ", "одна\nдругая", "таб\tтаб", strings.Repeat("я", 101)} {
		_, err := service.Create(context.Background(), 3, name)
		mustInvalid(t, err, "creating collection named "+name)
	}

	_, err := service.Create(context.Background(), 3, strings.Repeat("я", 100))
	must(t, err, "creating collection with the longest name")
}

func TestCollectionServiceUpdate(t *testing.T) {
	stub := &collectionRepositoryStub{
		byID:    domain.Collection{ID: 7, OwnerID: 3},
		updated: domain.Collection{ID: 7, Name: "Новая", IsPublic: true},
	}
	service := newCollectionService(stub)

	collection, err := service.Update(context.Background(), 3, 7, " Новая ", true)
	must(t, err, "updating collection")
	mustEqual(t, collection.Name, "Новая", "collection name")
	mustEqual(t, stub.receivedName, "Новая", "trimmed name")
	mustEqual(t, stub.receivedPub, true, "collection visibility")

	_, err = service.Update(context.Background(), 3, 7, "", true)
	mustInvalid(t, err, "updating collection with an empty name")
}

func TestCollectionServiceUpdateRejectsForeign(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3}}
	service := newCollectionService(stub)

	_, err := service.Update(context.Background(), 4, 7, "Новая", true)
	mustForbidden(t, err, "updating a foreign collection")
}

func TestCollectionServiceUpdateRepositoryError(t *testing.T) {
	stub := &collectionRepositoryStub{
		byID:      domain.Collection{ID: 7, OwnerID: 3},
		updateErr: errors.New("boom"),
	}
	service := newCollectionService(stub)

	_, err := service.Update(context.Background(), 3, 7, "Новая", true)
	mustFail(t, err, "updating collection with a repository error")
}

func TestCollectionServiceDelete(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3}}
	service := newCollectionService(stub)

	must(t, service.Delete(context.Background(), 3, 7), "deleting collection")
	mustEqual(t, stub.receivedID, int64(7), "collection id")

	stub.deleteErr = errors.New("boom")
	mustFail(t, service.Delete(context.Background(), 3, 7), "deleting collection with a repository error")
}

func TestCollectionServiceDeleteRejectsForeign(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3}}
	service := newCollectionService(stub)

	mustForbidden(t, service.Delete(context.Background(), 4, 7), "deleting a foreign collection")
}

func TestCollectionServiceAddAnekdot(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3}}
	service := newCollectionService(stub)

	must(t, service.AddAnekdot(context.Background(), 3, 7, 21), "adding anecdote")
	mustEqual(t, stub.receivedAnek, int64(21), "anecdote id")

	mustInvalid(t, service.AddAnekdot(context.Background(), 3, 7, 0), "adding a zero anecdote")

	stub.addErr = errors.New("boom")
	mustFail(t, service.AddAnekdot(context.Background(), 3, 7, 21), "adding anecdote with a repository error")

	mustForbidden(t, service.AddAnekdot(context.Background(), 4, 7, 21), "adding to a foreign collection")
}

func TestCollectionServiceRemoveAnekdot(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3}}
	service := newCollectionService(stub)

	must(t, service.RemoveAnekdot(context.Background(), 3, 7, 21), "removing anecdote")
	mustEqual(t, stub.receivedAnek, int64(21), "anecdote id")

	mustInvalid(t, service.RemoveAnekdot(context.Background(), 3, 7, 0), "removing a zero anecdote")

	stub.removeErr = errors.New("boom")
	mustFail(t, service.RemoveAnekdot(context.Background(), 3, 7, 21), "removing anecdote with a repository error")

	mustForbidden(t, service.RemoveAnekdot(context.Background(), 4, 7, 21), "removing from a foreign collection")
}

func TestCollectionServiceAddFavorite(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3, IsPublic: true}}
	service := newCollectionService(stub)

	must(t, service.AddFavorite(context.Background(), 5, 7), "favoriting collection")
	mustEqual(t, stub.receivedUser, int64(5), "user id")
	mustEqual(t, stub.receivedID, int64(7), "collection id")

	stub.favoriteErr = errors.New("boom")
	mustFail(t, service.AddFavorite(context.Background(), 5, 7), "favoriting collection with a repository error")
}

func TestCollectionServiceAddFavoriteRejectsPrivate(t *testing.T) {
	stub := &collectionRepositoryStub{byID: domain.Collection{ID: 7, OwnerID: 3, IsPublic: false}}
	service := newCollectionService(stub)

	mustForbidden(t, service.AddFavorite(context.Background(), 5, 7), "favoriting a private collection")
}

func TestCollectionServiceRemoveFavorite(t *testing.T) {
	stub := &collectionRepositoryStub{}
	service := newCollectionService(stub)

	must(t, service.RemoveFavorite(context.Background(), 5, 7), "unfavoriting collection")
	mustEqual(t, stub.receivedUser, int64(5), "user id")

	stub.unfavoriteErr = errors.New("boom")
	mustFail(t, service.RemoveFavorite(context.Background(), 5, 7), "unfavoriting with a repository error")
}

func TestNormalizeCollectionName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "plain", input: "Коллекция", want: "Коллекция"},
		{name: "trimmed", input: "\t Коллекция \n", want: "Коллекция"},
		{name: "inner space", input: "Моя коллекция", want: "Моя коллекция"},
		{name: "empty", input: "", wantErr: true},
		{name: "spaces", input: "    ", wantErr: true},
		{name: "newline", input: "две\nстроки", wantErr: true},
		{name: "carriage return", input: "две\rстроки", wantErr: true},
		{name: "tab", input: "две\tстроки", wantErr: true},
		{name: "too long", input: strings.Repeat("я", 101), wantErr: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeCollectionName(test.input)

			if test.wantErr {
				mustInvalid(t, err, "normalizing "+test.name)

				return
			}

			must(t, err, "normalizing "+test.name)
			mustEqual(t, got, test.want, "normalized name")
		})
	}
}

func TestViewerPointer(t *testing.T) {
	mustEqual(t, viewerPointer(0), (*int64)(nil), "zero viewer")
	mustEqual(t, viewerPointer(-2), (*int64)(nil), "negative viewer")
	mustEqual(t, viewerPointer(1) != nil && *viewerPointer(1) == 1, true, "smallest viewer")
	mustEqual(t, viewerPointer(4) != nil && *viewerPointer(4) == 4, true, "positive viewer")
}
