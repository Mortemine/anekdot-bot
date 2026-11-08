package domain

import "time"

type Category struct {
	ID   int
	Name string
}

type CategorySummary struct {
	Category
	AnekdotCount int
}

type Anekdot struct {
	ID            int64
	CategoryID    int
	CategoryName  string
	Text          string
	AuthorID      *int64
	CreatedAt     time.Time
	LikeCount     int
	FavoriteCount int
}

type AnekdotSort string

const (
	AnekdotSortCreated   AnekdotSort = "created"
	AnekdotSortLikes     AnekdotSort = "likes"
	AnekdotSortFavorites AnekdotSort = "favorites"
)

type AnekdotPage struct {
	Anekdots []Anekdot
	Total    int
	Limit    int
	Offset   int
}

type User struct {
	ID          int64
	Email       string
	DisplayName string
	CreatedAt   time.Time
}

type Credentials struct {
	User
	PasswordHash string
}

type Session struct {
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
}
