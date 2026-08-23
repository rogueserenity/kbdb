package handlers

import (
	"errors"
	"net/http"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// handleMutationError is the standard error tail for a mutating repository
// call: repository.ErrNotFound -> 404, repository.ErrMutationConflict ->
// 409 (logged as a warning), any other non-nil error -> 500 (logged as an
// error). logFields are passed through to the log call for correlation
// (e.g. log.SwitchID, id). Returns true if err was non-nil and a response
// was written - callers should return immediately when true.
func handleMutationError(w http.ResponseWriter, r *http.Request, err error, logFields ...any) bool {
	if errors.Is(err, repository.ErrNotFound) {
		problem.NotFound(w, "resource not found")
		return true
	}
	if errors.Is(err, repository.ErrMutationConflict) {
		log.FromContext(r.Context()).Warn("mutation conflict", logFields...)
		problem.Conflict(w, "the resource is being modified concurrently, please retry")
		return true
	}
	if err != nil {
		log.FromContext(r.Context()).Error("mutation failed", append([]any{log.Error, err}, logFields...)...)
		problem.Internal(w, "an internal error occurred")
		return true
	}
	return false
}
