package external

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
)

const (
	maxResponseBytes = 1 << 20
	userAgent        = "anekdot-bot/1.0"

	dialTimeout         = 5 * time.Second
	keepAlive           = 30 * time.Second
	maxIdleConns        = 10
	maxIdleConnsPerHost = 5
	idleConnTimeout     = 30 * time.Second
	tlsHandshakeTimeout = 5 * time.Second
)

type BaneksSource struct {
	url    string
	client *http.Client
	logger *slog.Logger
}

func NewBaneksSource(rawURL string, timeout time.Duration, logger *slog.Logger) (*BaneksSource, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing external anecdote url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("external anecdote url must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("external anecdote url is missing a host")
	}

	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: timeout,
	}

	return &BaneksSource{
		url:    parsed.String(),
		client: &http.Client{Timeout: timeout, Transport: transport},
		logger: logger,
	}, nil
}

func (s *BaneksSource) Random(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return "", fmt.Errorf("building external request: %w", err)
	}
	request.Header.Set("Accept", "text/html")
	request.Header.Set("User-Agent", userAgent)

	response, err := s.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("%w: fetching external anecdote: %w", domain.ErrUpstreamUnavailable, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: external provider returned status %d", domain.ErrUpstreamUnavailable, response.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: reading external response: %w", domain.ErrUpstreamUnavailable, err)
	}
	if len(body) > maxResponseBytes {
		return "", fmt.Errorf("%w: external response is too large", domain.ErrUpstreamUnavailable)
	}

	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%w: parsing external response: %w", domain.ErrUpstreamUnavailable, err)
	}

	text := textWithLineBreaks(document.Find("p").First())
	if text == "" {
		return "", fmt.Errorf("%w: external provider returned no anecdote", domain.ErrUpstreamUnavailable)
	}

	s.logger.DebugContext(ctx, "external anecdote fetched", "length", len(text))

	return text, nil
}

func textWithLineBreaks(selection *goquery.Selection) string {
	var builder strings.Builder

	selection.Contents().Each(func(_ int, node *goquery.Selection) {
		if goquery.NodeName(node) == "br" {
			builder.WriteByte('\n')

			return
		}

		builder.WriteString(node.Text())
	})

	return strings.TrimSpace(builder.String())
}
