// Package handlers holds the REST route handler functions, one file per
// entity. Each handler takes its repository interface from
// [github.com/rogueserenity/kbdb/internal/repository], maps to and from the
// spec-shaped types via [github.com/rogueserenity/kbdb/internal/repoapi],
// and reports failures as RFC 9457 responses via
// [github.com/rogueserenity/kbdb/internal/problem]. Router wiring lives in
// [github.com/rogueserenity/kbdb/internal/router], not here.
package handlers
