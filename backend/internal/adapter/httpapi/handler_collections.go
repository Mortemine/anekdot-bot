package httpapi

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/alexey/anekdotus/backend/internal/domain"
	"github.com/alexey/anekdotus/backend/internal/usecase"
)

const maxCollectionBodyBytes = 8 << 10

type collectionDTO struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	OwnerID         int64     `json:"owner_id"`
	OwnerName       string    `json:"owner_name"`
	IsPublic        bool      `json:"is_public"`
	AnekdotCount    int       `json:"anekdot_count"`
	ContainsAnekdot bool      `json:"contains_anekdot"`
	Favorited       bool      `json:"favorited"`
	CreatedAt       time.Time `json:"created_at"`
}

type collectionsResponse struct {
	Collections []collectionDTO `json:"collections"`
}

type collectionDetailResponse struct {
	Collection collectionDTO `json:"collection"`
	Anekdots   []anekdotDTO  `json:"anekdots"`
}

type createCollectionRequest struct {
	Name string `json:"name"`
}

type updateCollectionRequest struct {
	Name     string `json:"name"`
	IsPublic bool   `json:"is_public"`
}

type CollectionHandler struct {
	service *usecase.CollectionService
	logger  *slog.Logger
}

func NewCollectionHandler(service *usecase.CollectionService, logger *slog.Logger) *CollectionHandler {
	return &CollectionHandler{service: service, logger: logger}
}

func (h *CollectionHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	anekdotID, ok := parseFilterID(w, r.URL.Query().Get("anekdot_id"), "анекдота")
	if !ok {
		return
	}

	collections, err := h.service.Mine(r.Context(), user.ID, anekdotID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, collectionsResponse{Collections: toCollectionsDTO(collections)})
}

func (h *CollectionHandler) ListPublic(w http.ResponseWriter, r *http.Request) {
	anekdotID, ok := parseFilterID(w, r.URL.Query().Get("anekdot_id"), "анекдота")
	if !ok {
		return
	}

	collections, err := h.service.Public(r.Context(), currentUserID(r), anekdotID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, collectionsResponse{Collections: toCollectionsDTO(collections)})
}

func (h *CollectionHandler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collections, err := h.service.Favorites(r.Context(), user.ID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, collectionsResponse{Collections: toCollectionsDTO(collections)})
}

func (h *CollectionHandler) Detail(w http.ResponseWriter, r *http.Request) {
	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return
	}

	detail, err := h.service.Detail(r.Context(), currentUserID(r), collectionID)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, collectionDetailResponse{
		Collection: toCollectionDTO(detail.Collection),
		Anekdots:   toAnekdotsDTO(detail.Anekdots),
	})
}

func (h *CollectionHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	var request createCollectionRequest
	if !decodeRequest(w, r, maxCollectionBodyBytes, &request) {
		return
	}

	collection, err := h.service.Create(r.Context(), user.ID, request.Name)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusCreated, collectionDetailResponse{
		Collection: toCollectionDTO(collection),
		Anekdots:   []anekdotDTO{},
	})
}

func (h *CollectionHandler) Update(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return
	}

	var request updateCollectionRequest
	if !decodeRequest(w, r, maxCollectionBodyBytes, &request) {
		return
	}

	collection, err := h.service.Update(r.Context(), user.ID, collectionID, request.Name, request.IsPublic)
	if err != nil {
		h.handleError(w, r, err)

		return
	}

	writeJSON(w, http.StatusOK, collectionDetailResponse{
		Collection: toCollectionDTO(collection),
		Anekdots:   []anekdotDTO{},
	})
}

func (h *CollectionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), user.ID, collectionID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) AddAnekdot(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, anekdotID, ok := h.collectionAnekdotIDs(w, r)
	if !ok {
		return
	}

	if err := h.service.AddAnekdot(r.Context(), user.ID, collectionID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) RemoveAnekdot(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, anekdotID, ok := h.collectionAnekdotIDs(w, r)
	if !ok {
		return
	}

	if err := h.service.RemoveAnekdot(r.Context(), user.ID, collectionID, anekdotID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) AddFavorite(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return
	}

	if err := h.service.AddFavorite(r.Context(), user.ID, collectionID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) {
	user, ok := requireUser(w, r)
	if !ok {
		return
	}

	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return
	}

	if err := h.service.RemoveFavorite(r.Context(), user.ID, collectionID); err != nil {
		h.handleError(w, r, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *CollectionHandler) collectionAnekdotIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	collectionID, ok := parsePathID(w, r, "collection_id", "коллекции")
	if !ok {
		return 0, 0, false
	}

	anekdotID, ok := parsePathID(w, r, "anekdot_id", "анекдота")
	if !ok {
		return 0, 0, false
	}

	return collectionID, anekdotID, true
}

func (h *CollectionHandler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	writeRequestError(w, r, h.logger, "collection request failed", err)
}

func toCollectionsDTO(collections []domain.Collection) []collectionDTO {
	result := make([]collectionDTO, 0, len(collections))
	for _, collection := range collections {
		result = append(result, toCollectionDTO(collection))
	}

	return result
}

func toCollectionDTO(collection domain.Collection) collectionDTO {
	return collectionDTO{
		ID:              collection.ID,
		Name:            collection.Name,
		OwnerID:         collection.OwnerID,
		OwnerName:       collection.OwnerName,
		IsPublic:        collection.IsPublic,
		AnekdotCount:    collection.AnekdotCount,
		ContainsAnekdot: collection.ContainsAnekdot,
		Favorited:       collection.Favorited,
		CreatedAt:       collection.CreatedAt,
	}
}

func currentUserID(r *http.Request) int64 {
	user, ok := UserFromContext(r.Context())
	if !ok {
		return 0
	}

	return user.ID
}

func parsePathID(w http.ResponseWriter, r *http.Request, name, label string) (int64, bool) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value <= 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_input",
			"Идентификатор "+label+" должен быть положительным числом.",
		)

		return 0, false
	}

	return value, true
}

func parseFilterID(w http.ResponseWriter, raw, label string) (int64, bool) {
	if raw == "" {
		return 0, true
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		writeError(
			w,
			http.StatusBadRequest,
			"invalid_input",
			"Идентификатор "+label+" должен быть положительным числом.",
		)

		return 0, false
	}

	return value, true
}
