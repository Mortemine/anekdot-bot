package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/alexey/anekdot-bot/backend/internal/domain"
	"github.com/alexey/anekdot-bot/backend/internal/usecase"
)

type contextKey string

const (
	requestIDContextKey contextKey = "request_id"

	limiterEntryTTL   = 10 * time.Minute
	maxTrackedClients = 10_000
)

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey).(string)

	return requestID
}

func chain(handler http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for index := len(middleware) - 1; index >= 0; index-- {
		handler = middleware[index](handler)
	}

	return handler
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" || len(requestID) > 128 {
			requestID = newRequestID()
		}

		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}

	return hex.EncodeToString(buffer[:])
}

func recovererMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				if err, ok := recovered.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(recovered)
				}

				logger.ErrorContext(
					ctx,
					"panic recovered",
					"panic", recovered,
					"stack", string(debug.Stack()),
					"request_id", requestIDFromContext(ctx),
				)

				writeError(w, http.StatusInternalServerError, "internal_error", "Внутренняя ошибка сервера.")
			}()

			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	written, err := r.ResponseWriter.Write(body)
	r.bytes += written

	return written, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func accessLogMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			ctx := r.Context()

			defer func() {
				logger.InfoContext(
					ctx,
					"http request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", recorder.status,
					"bytes", recorder.bytes,
					"duration_ms", time.Since(start).Milliseconds(),
					"request_id", requestIDFromContext(ctx),
				)
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

const contentSecurityPolicy = "default-src 'self'; " +
	"img-src 'self' data:; " +
	"style-src 'self'; " +
	"script-src 'self'; " +
	"connect-src 'self'; " +
	"font-src 'self'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		headers.Set("Content-Security-Policy", contentSecurityPolicy)

		if r.TLS != nil {
			headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(origins))
	allowAll := false
	for _, origin := range origins {
		if origin == "*" {
			allowAll = true

			continue
		}
		allowed[origin] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			_, isAllowed := allowed[origin]

			switch {
			case allowAll:
				w.Header().Set("Access-Control-Allow-Origin", "*")
			case origin != "" && isAllowed:
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set(
					"Access-Control-Allow-Headers",
					"Content-Type, Authorization, X-Admin-Token, X-Request-Id",
				)
				w.Header().Set("Access-Control-Max-Age", "600")
			default:
				next.ServeHTTP(w, r)

				return
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type userContextKey struct{}

func UserFromContext(ctx context.Context) (domain.User, bool) {
	user, ok := ctx.Value(userContextKey{}).(domain.User)

	return user, ok
}

func bearerToken(r *http.Request) string {
	const prefix = "bearer "

	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}

	return strings.TrimSpace(header[len(prefix):])
}

func authMiddleware(service *usecase.AuthService, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := service.Authenticate(r.Context(), bearerToken(r))
			if err != nil {
				if !errors.Is(err, domain.ErrUnauthorized) {
					logger.ErrorContext(
						r.Context(),
						"session lookup failed",
						"error", err,
						"request_id", requestIDFromContext(r.Context()),
					)
				}

				w.Header().Set("WWW-Authenticate", `Bearer realm="anekdot"`)
				writeError(w, http.StatusUnauthorized, "unauthorized", "Требуется вход в аккаунт.")

				return
			}

			ctx := context.WithValue(r.Context(), userContextKey{}, user)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type ipLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu         sync.Mutex
	entries    map[string]*ipLimiterEntry
	limit      rate.Limit
	burst      int
	entryTTL   time.Duration
	maxEntries int
}

func newIPRateLimiter(limit rate.Limit, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		entries:    map[string]*ipLimiterEntry{},
		limit:      limit,
		burst:      burst,
		entryTTL:   limiterEntryTTL,
		maxEntries: maxTrackedClients,
	}
}

func (l *ipRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.evictStale(now)

	entry, ok := l.entries[key]
	if !ok {
		entry = &ipLimiterEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = entry
	}
	entry.lastSeen = now

	return entry.limiter.Allow()
}

func (l *ipRateLimiter) evictStale(now time.Time) {
	if len(l.entries) < l.maxEntries {
		return
	}

	for key, entry := range l.entries {
		if now.Sub(entry.lastSeen) > l.entryTTL {
			delete(l.entries, key)
		}
	}
}

func rateLimitMiddleware(limiter *ipRateLimiter, trustProxy bool, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, trustProxy)
			if !limiter.Allow(ip) {
				logger.WarnContext(
					r.Context(),
					"rate limit exceeded",
					"ip", ip,
					"request_id", requestIDFromContext(r.Context()),
				)
				w.Header().Set("Retry-After", "60")
				writeError(w, http.StatusTooManyRequests, "rate_limited", "Слишком много запросов. Попробуйте позже.")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if first != "" {
				return first
			}
		}
		if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}
