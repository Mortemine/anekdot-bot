package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

func TestClampPageSize(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		limit int
		want  int
	}{
		"negative":      {limit: -1, want: categoryPageSize},
		"zero":          {limit: 0, want: categoryPageSize},
		"one":           {limit: 1, want: 1},
		"page size":     {limit: categoryPageSize, want: categoryPageSize},
		"below maximum": {limit: maxCategoryPageSize - 1, want: maxCategoryPageSize - 1},
		"at maximum":    {limit: maxCategoryPageSize, want: maxCategoryPageSize},
		"above maximum": {limit: maxCategoryPageSize + 1, want: maxCategoryPageSize},
		"far above it":  {limit: 10_000, want: maxCategoryPageSize},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := clampPageSize(testCase.limit); got != testCase.want {
				t.Fatalf("clampPageSize(%d) = %d, want %d", testCase.limit, got, testCase.want)
			}
		})
	}
}

func TestNormalizeAnekdotText(t *testing.T) {
	t.Parallel()

	tooLong := strings.Repeat("я", maxAnekdotLength)
	oneTooLong := strings.Repeat("я", maxAnekdotLength+1)

	cases := map[string]struct {
		text    string
		want    string
		wantErr bool
	}{
		"trimmed":        {text: "  Штирлиц шел по коридору  ", want: "Штирлиц шел по коридору"},
		"too short":      {text: strings.Repeat("я", minAnekdotLength-1), wantErr: true},
		"at minimum":     {text: strings.Repeat("я", minAnekdotLength), want: strings.Repeat("я", minAnekdotLength)},
		"at maximum":     {text: tooLong, want: tooLong},
		"above maximum":  {text: oneTooLong, wantErr: true},
		"empty":          {text: "   ", wantErr: true},
		"trimmed below":  {text: "   коротко   ", wantErr: true},
		"trimmed to max": {text: "  " + tooLong + "  ", want: tooLong},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeAnekdotText(testCase.text)
			if testCase.wantErr {
				if !errors.Is(err, domain.ErrInvalidInput) {
					t.Fatalf("expected ErrInvalidInput, got %v", err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != testCase.want {
				t.Fatalf("got %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestCheckAnekdotID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		id      int64
		wantErr bool
	}{
		"negative": {id: -1, wantErr: true},
		"zero":     {id: 0, wantErr: true},
		"one":      {id: 1},
		"large":    {id: 1 << 40},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := checkAnekdotID(testCase.id)
			if testCase.wantErr != errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("checkAnekdotID(%d) = %v, wantErr %v", testCase.id, err, testCase.wantErr)
			}
		})
	}
}

func TestCheckCategory(t *testing.T) {
	t.Parallel()

	t.Run("negative is rejected before lookup", func(t *testing.T) {
		t.Parallel()

		service := newTestService(serviceDeps{
			categories: &categoryRepositoryStub{getErr: domain.ErrNotFound},
		})

		err := service.checkCategory(context.Background(), -1)
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}

		if errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("category lookup ran for a negative id: %v", err)
		}
	})

	t.Run("zero is a valid category", func(t *testing.T) {
		t.Parallel()

		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{}})

		if err := service.checkCategory(context.Background(), 0); err != nil {
			t.Fatalf("category 0 must be accepted, got %v", err)
		}
	})

	t.Run("one is a valid category", func(t *testing.T) {
		t.Parallel()

		service := newTestService(serviceDeps{categories: &categoryRepositoryStub{}})

		if err := service.checkCategory(context.Background(), 1); err != nil {
			t.Fatalf("category 1 must be accepted, got %v", err)
		}
	})

	t.Run("missing category is reported", func(t *testing.T) {
		t.Parallel()

		service := newTestService(serviceDeps{
			categories: &categoryRepositoryStub{getErr: domain.ErrNotFound},
		})

		if err := service.checkCategory(context.Background(), 7); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
