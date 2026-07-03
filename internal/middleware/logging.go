// Package middleware provides net/http middleware: structured request
// logging and token-based authentication.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type contextKey int

const loggerContextKey contextKey = iota

// FromContext returns the request-scoped logger stored by Middleware, or the
// default logger if none is present (e.g. outside a request, in tests).
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerContextKey).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithUserID returns a context whose logger additionally carries user_id.
// Called once VerifyToken has resolved the caller's identity.
func WithUserID(ctx context.Context, userID string) context.Context {
	logger := FromContext(ctx).With("user_id", userID)
	return context.WithValue(ctx, loggerContextKey, logger)
}

// Logging generates a request ID, builds a request-scoped slog.Logger, and
// logs once at request start and once at request end (status/duration).
// Per-internal-call logging is deliberately not done here to avoid noise.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		logger := slog.Default().With("request_id", requestID)
		ctx := context.WithValue(r.Context(), loggerContextKey, logger)
		r = r.WithContext(ctx)

		logger.Info("request started", "method", r.Method, "path", r.URL.Path)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		logger.Info("request finished",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
