// Package authz handles authorization: deciding what an already-identified
// caller (see internal/auth) may read or write. It does not verify tokens
// or establish identity itself.
package authz

import (
	"context"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// Owned is implemented by entity structs that carry an owner and a
// visibility tier.
type Owned interface {
	OwnerID() string
	VisibilityTier() repository.Visibility
}

// CanRead reports whether the caller identified by ctx (anonymous if no
// user ID is set) may read item, per its owner and visibility tier.
func CanRead(ctx context.Context, item Owned) bool {
	subject, _ := kbdbctx.UserID(ctx)
	if subject != "" && item.OwnerID() == subject {
		return true
	}

	switch item.VisibilityTier() {
	case repository.VisibilityPublic:
		return true
	case repository.VisibilityAuthenticated:
		return subject != ""
	case repository.VisibilityPrivate:
		return false
	default:
		return false
	}
}

// IsOwner reports whether the caller identified by ctx owns item. Used to
// gate writes, which are always owner-only regardless of visibility.
func IsOwner(ctx context.Context, item Owned) bool {
	subject, _ := kbdbctx.UserID(ctx)
	return subject != "" && item.OwnerID() == subject
}
