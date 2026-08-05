package log

import (
	"context"
	"log/slog"
)

// Structured logging field names, shared across internal/handlers so the
// same concept (e.g. "the resource's own ID") always logs under the same
// key regardless of which handler emits the line.
const (
	Error          = "error"
	KeyboardID     = "keyboard_id"
	SwitchID       = "switch_id"
	KeycapSetID    = "keycap_set_id"
	KeycapKitID    = "keycap_kit_id"
	KeycapKitImage = "keycap_kit_image"
	LookupCategory = "lookup_category"
)

type loggerKey struct{}

// New returns a base logger with no request-specific fields yet.
func New() *slog.Logger {
	return slog.Default()
}

// WithRequestID returns l with a request_id field added.
func WithRequestID(l *slog.Logger, requestID string) *slog.Logger {
	return l.With("request_id", requestID)
}

// WithUserID returns l with a user_id field added.
func WithUserID(l *slog.Logger, userID string) *slog.Logger {
	return l.With("user_id", userID)
}

// WithLogger returns a context carrying l as the request-scoped logger.
func WithLogger(c context.Context, l *slog.Logger) context.Context {
	return context.WithValue(c, loggerKey{}, l)
}

// FromContext returns the request-scoped logger stored by WithLogger, or the
// default logger if none is present (e.g. outside a request, in tests).
func FromContext(c context.Context) *slog.Logger {
	if l, ok := c.Value(loggerKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
