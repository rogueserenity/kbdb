// Package middleware provides net/http middleware: structured request
// logging and token-based authentication.
package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	logpkg "github.com/rogueserenity/kbdb/internal/log"
)

// Logging generates a request ID, builds a request-scoped slog.Logger, and
// logs once at request start and once at request end (status/duration).
// Per-internal-call logging is deliberately not done here to avoid noise.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		l := logpkg.WithRequestID(logpkg.New(), requestID)
		ctx := logpkg.WithLogger(r.Context(), l)
		r = r.WithContext(ctx)

		l.Info("request started", "method", r.Method, "path", r.URL.Path)

		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		l.Info("request finished",
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
