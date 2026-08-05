// Package repoapi maps between internal/repository's DB-shaped types and
// internal/handlers/api's generated, spec-shaped types. Neither of those
// packages imports the other; this package imports both so the mapping is
// defined once per entity instead of inline in each handler.
package repoapi
