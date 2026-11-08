package external

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alexey/anekdotus/backend/internal/domain"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewBaneksSourceRejectsInvalidURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{name: "empty", url: ""},
		{name: "wrong scheme", url: "ftp://example.com/random"},
		{name: "missing host", url: "https://"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewBaneksSource(test.url, time.Second, discardLogger()); err == nil {
				t.Fatal("NewBaneksSource() error = nil, want error")
			}
		})
	}
}

func TestBaneksSourceRandom(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    string
		wantErr error
	}{
		{
			name:   "line breaks",
			status: http.StatusOK,
			body:   `<html><body><p>Первая<br>Вторая</p></body></html>`,
			want:   "Первая\nВторая",
		},
		{
			name:   "plain text",
			status: http.StatusOK,
			body:   `<p>  Просто анекдот  </p>`,
			want:   "Просто анекдот",
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			body:    "boom",
			wantErr: domain.ErrUpstreamUnavailable,
		},
		{
			name:    "missing paragraph",
			status:  http.StatusOK,
			body:    `<div>нет анекдота</div>`,
			wantErr: domain.ErrUpstreamUnavailable,
		},
		{
			name:    "empty paragraph",
			status:  http.StatusOK,
			body:    `<p>   </p>`,
			wantErr: domain.ErrUpstreamUnavailable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, test.body)
			}))
			defer server.Close()

			source, err := NewBaneksSource(server.URL, time.Second, discardLogger())
			if err != nil {
				t.Fatalf("NewBaneksSource() error = %v", err)
			}

			text, err := source.Random(context.Background())
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("Random() error = %v, want %v", err, test.wantErr)
				}

				return
			}
			if err != nil {
				t.Fatalf("Random() error = %v", err)
			}
			if text != test.want {
				t.Errorf("Random() = %q, want %q", text, test.want)
			}
		})
	}
}

func TestTextWithLineBreaks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<p>Один<br/>Два<br>Три</p>`)
	}))
	defer server.Close()

	source, err := NewBaneksSource(server.URL, time.Second, discardLogger())
	if err != nil {
		t.Fatalf("NewBaneksSource() error = %v", err)
	}

	text, err := source.Random(context.Background())
	if err != nil {
		t.Fatalf("Random() error = %v", err)
	}
	if text != "Один\nДва\nТри" {
		t.Errorf("Random() = %q", text)
	}
}
