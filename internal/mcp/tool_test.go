package mcp

import (
	"context"
	"testing"

	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
)

const (
	callerID = "caller-0001"
	otherID  = "other-0002"
)

// callerContext mimics what identityMiddleware puts on ctx for a verified
// caller, since these handlers run downstream of it.
func callerContext(t *testing.T) context.Context {
	t.Helper()

	return ctxpkg.WithUserID(t.Context(), callerID)
}
