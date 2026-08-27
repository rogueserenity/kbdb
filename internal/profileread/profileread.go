// Package profileread resolves a GET /v1/profile/{identifier} / get_profile
// identifier (an IdP subject or a username) to a profile and applies the
// discoverable-or-owner visibility rule. It's a separate package so both
// internal/handlers and internal/mcp can share the exact same logic without
// one importing the other.
package profileread

import (
	"context"
	"errors"
	"strings"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// Resolve looks up the profile named by identifier and returns it only if
// it may be shown to the caller.
//
// Resolution is id-first, username-fallback: identifier is tried as an IdP
// subject (repo.Get); on ErrNotFound it's tried as a username
// (repo.ResolveUsername, then repo.Get). The username charset is disjoint
// from IdP subject values, so the id-first probe never matches a profile
// that was meant to be found by username.
//
// Visibility: the profile is returned only if it's discoverable, or if the
// caller (from ctx) is its owner. Otherwise - and when nothing matches
// identifier at all - Resolve returns (nil, false, nil): "not found", with
// no error, so callers respond 404 without leaking whether a
// non-discoverable profile exists. A non-nil error means an actual failure
// (a store error), not "not found".
func Resolve(ctx context.Context, repo repository.ProfileRepository, identifier string) (*repository.Profile, bool, error) {
	p, err := repo.Get(ctx, identifier)
	if errors.Is(err, repository.ErrNotFound) {
		subject, resolveErr := repo.ResolveUsername(ctx, strings.ToLower(identifier))
		if errors.Is(resolveErr, repository.ErrNotFound) {
			return nil, false, nil
		}
		if resolveErr != nil {
			return nil, false, resolveErr
		}

		p, err = repo.Get(ctx, subject)
		if errors.Is(err, repository.ErrNotFound) {
			// The claim item points at a subject with no profile - treat as
			// not found rather than an error; a later issue's delete path
			// keeps the two in sync, but a stale claim must not 500.
			return nil, false, nil
		}
	}
	if err != nil {
		return nil, false, err
	}

	if p.Discoverable {
		return p, true, nil
	}

	if caller, ok := kbdbctx.UserID(ctx); ok && caller == p.StytchUserID {
		return p, true, nil
	}

	return nil, false, nil
}
