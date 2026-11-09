package domain

import (
	"errors"
	"testing"
)

func TestParseAnekdotSort(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    AnekdotSort
		wantErr bool
	}{
		{name: "empty falls back to created", raw: "", want: AnekdotSortCreated},
		{name: "created", raw: "created", want: AnekdotSortCreated},
		{name: "likes", raw: "likes", want: AnekdotSortLikes},
		{name: "favorites", raw: "favorites", want: AnekdotSortFavorites},
		{name: "unknown", raw: "popular", wantErr: true},
		{name: "case sensitive", raw: "Likes", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseAnekdotSort(test.raw)
			if test.wantErr {
				if err == nil {
					t.Fatalf("ParseAnekdotSort(%q) error = nil, want error", test.raw)
				}

				if !errors.Is(err, ErrInvalidInput) {
					t.Errorf("error = %v, want it to wrap ErrInvalidInput", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseAnekdotSort(%q) error = %v", test.raw, err)
			}

			if got != test.want {
				t.Errorf("ParseAnekdotSort(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}
