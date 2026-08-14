package repository

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned by repository methods when the requested item
// does not exist. Handlers check for it via errors.Is to return 404 instead
// of a generic 500.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned by repository methods when a create
// operation targets an item that already exists. Handlers check for it via
// errors.Is to return 409 instead of a generic 500.
var ErrAlreadyExists = errors.New("already exists")

// ErrMutationConflict is returned when a repository method that
// read-modify-writes an item (e.g. KeycapSetRepository's kit/set mutations)
// loses a bounded number of optimistic-concurrency retries in a row -
// concurrent writers keep winning the race. Handlers check for it via
// errors.Is to return 409 instead of a generic 500: the client's fix is to
// retry the whole request, not to change anything about it.
var ErrMutationConflict = errors.New("mutation conflict: too many concurrent writers")

// ErrNoUserID guards every Create/Update/Delete against a silently ignored
// internal/ctx.UserID ok - an empty-string partition key or object-store key
// would otherwise write against user_id="" instead of erroring.
var ErrNoUserID = errors.New("no user id in context")

// ErrStillReferenced is returned by a Delete/DeleteKit call in "block" mode
// (the default) when the item is still referenced by at least one Build.
// Handlers check for it via errors.Is to return 409 instead of proceeding.
// Callers that need the specific blocking build ids should use errors.As
// with *StillReferencedError instead.
var ErrStillReferenced = errors.New("still referenced by at least one build")

// StillReferencedError carries the ids of the builds blocking a delete.
// It matches errors.Is(err, ErrStillReferenced), so callers that only need
// the general check can keep using that, while callers that need the ids
// (e.g. to populate a 409 response body) use errors.As.
type StillReferencedError struct {
	BuildIDs []string
}

func (e *StillReferencedError) Error() string {
	return fmt.Sprintf("still referenced by builds: %s", strings.Join(e.BuildIDs, ", "))
}

// Is matches ErrStillReferenced so errors.Is(err, ErrStillReferenced) works
// on a *StillReferencedError without callers needing errors.As.
func (e *StillReferencedError) Is(target error) bool {
	return target == ErrStillReferenced
}
