// Package log holds the request-scoped *slog.Logger-in-context plumbing and
// logger field transformations, independent of any particular transport
// (REST, MCP) and independent of internal/ctx — this package has no
// knowledge of where field values (request ID, user ID) come from; callers
// pass them in explicitly.
package log
