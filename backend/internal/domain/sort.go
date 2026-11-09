package domain

import "fmt"

type AnekdotSort string

const (
	AnekdotSortCreated   AnekdotSort = "created"
	AnekdotSortLikes     AnekdotSort = "likes"
	AnekdotSortFavorites AnekdotSort = "favorites"
)

func ParseAnekdotSort(raw string) (AnekdotSort, error) {
	switch AnekdotSort(raw) {
	case "":
		return AnekdotSortCreated, nil
	case AnekdotSortCreated, AnekdotSortLikes, AnekdotSortFavorites:
		return AnekdotSort(raw), nil
	default:
		return "", fmt.Errorf("%w: sort must be created, likes or favorites", ErrInvalidInput)
	}
}
