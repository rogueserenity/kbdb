package repository

import "errors"

// ErrNotFound is returned by repository methods when the requested item
// does not exist. Handlers check for it via errors.Is to return 404 instead
// of a generic 500.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned by repository methods when a create
// operation targets an item that already exists. Handlers check for it via
// errors.Is to return 409 instead of a generic 500.
var ErrAlreadyExists = errors.New("already exists")
