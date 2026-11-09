package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	exactlyMaxBytes := strings.Repeat("a", maxPasswordBytes)
	cases := map[string]struct {
		password string
		wantErr  bool
	}{
		"empty":            {password: "", wantErr: true},
		"too short":        {password: strings.Repeat("a", minPasswordLength-1), wantErr: true},
		"at minimum":       {password: strings.Repeat("a", minPasswordLength)},
		"long":             {password: strings.Repeat("a", maxPasswordBytes-1)},
		"at max bytes":     {password: exactlyMaxBytes},
		"above max bytes":  {password: exactlyMaxBytes + "a", wantErr: true},
		"multibyte short":  {password: strings.Repeat("я", minPasswordLength-1), wantErr: true},
		"multibyte at min": {password: strings.Repeat("я", minPasswordLength)},
		"multibyte long":   {password: strings.Repeat("я", maxPasswordBytes), wantErr: true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := validatePassword(testCase.password)
			if testCase.wantErr != errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("validatePassword(%d bytes) = %v, wantErr %v", len(testCase.password), err, testCase.wantErr)
			}
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     string
		want    string
		wantErr bool
	}{
		"trimmed and lowered": {raw: "  Reader@Example.COM  ", want: "reader@example.com"},
		"plain":               {raw: "reader@example.com", want: "reader@example.com"},
		"empty":               {raw: "", wantErr: true},
		"blank":               {raw: "   ", wantErr: true},
		"no at sign":          {raw: "reader.example.com", wantErr: true},
		"no domain":           {raw: "reader@", wantErr: true},
		"display name":        {raw: "Reader <reader@example.com>", wantErr: true},
		"too long":            {raw: strings.Repeat("a", maxEmailLength) + "@example.com", wantErr: true},
		"at max length":       {raw: strings.Repeat("a", maxEmailLength-len("@example.com")) + "@example.com", want: strings.Repeat("a", maxEmailLength-len("@example.com")) + "@example.com"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeEmail(testCase.raw)
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

func TestNormalizeDisplayName(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		raw     string
		email   string
		want    string
		wantErr bool
	}{
		"trimmed":        {raw: "  Читатель  ", email: "reader@example.com", want: "Читатель"},
		"from email":     {raw: "   ", email: "reader@example.com", want: "reader"},
		"empty email":    {raw: "", email: "reader", wantErr: true},
		"at maximum":     {raw: strings.Repeat("я", maxDisplayNameLength), email: "a@b.c", want: strings.Repeat("я", maxDisplayNameLength)},
		"above maximum":  {raw: strings.Repeat("я", maxDisplayNameLength+1), email: "a@b.c", wantErr: true},
		"email too long": {raw: "", email: strings.Repeat("a", maxDisplayNameLength+1) + "@b.c", wantErr: true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeDisplayName(testCase.raw, testCase.email)
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
