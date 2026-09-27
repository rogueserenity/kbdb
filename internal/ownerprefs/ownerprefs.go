package ownerprefs

import (
	"context"
	"errors"
	"sync"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ErrNoLoader means [Get] ran on a context [WithLoader] never touched - a
// route or tool missing its owner-preferences wiring.
var ErrNoLoader = errors.New("no owner preferences loader on context")

type loaderKey struct{}

type loader struct {
	reader  repository.PreferencesReader
	ownerID string

	once  sync.Once
	prefs repository.ProfilePreferences
	err   error
}

// WithLoader returns a context on which [Get] fetches ownerID's preferences
// from reader, once.
func WithLoader(ctx context.Context, reader repository.PreferencesReader, ownerID string) context.Context {
	return context.WithValue(ctx, loaderKey{}, &loader{reader: reader, ownerID: ownerID})
}

// Get returns the owner's preferences, fetching them on the first call and
// returning the same result, error included, on every call after.
func Get(ctx context.Context) (repository.ProfilePreferences, error) {
	l, ok := ctx.Value(loaderKey{}).(*loader)
	if !ok {
		return repository.ProfilePreferences{}, ErrNoLoader
	}

	l.once.Do(func() {
		l.prefs, l.err = l.reader.GetPreferences(ctx, l.ownerID)
	})

	return l.prefs, l.err
}
