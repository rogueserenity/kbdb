package repository

import (
	"errors"
)

// ErrNotFound is returned by repository methods when the requested item
// does not exist. Handlers check for it via errors.Is to return 404 instead
// of a generic 500.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned by repository methods when a create
// operation targets an item that already exists. Handlers check for it via
// errors.Is to return 409 instead of a generic 500.
var ErrAlreadyExists = errors.New("already exists")

// ErrUsernameTaken is returned by ProfileRepository create/update when the
// requested username is claimed by a *different* user - distinct from
// ErrAlreadyExists ("this user already has a profile"). Handlers map it to
// a 409 with the username-unavailable problem type so the frontend can
// surface it against the username field.
var ErrUsernameTaken = errors.New("username taken")

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
