// Package handlers holds the REST route handler functions, one file per
// entity. Each handler takes its repository interface from
// internal/repository, maps to and from the spec-shaped types via
// internal/repoapi, and reports failures as RFC 9457 responses via
// internal/problem. Router wiring lives in internal/router, not here.
package handlers
