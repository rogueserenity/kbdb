// Package authz handles authorization: deciding what an already-identified
// caller (see internal/auth) may read or write. It does not verify tokens
// or establish identity itself.
package authz

import (
	"context"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ReadableVisibilities reports which visibility tiers the caller identified
// by ctx (anonymous if no user ID is set) may read, for a route whose path
// names ownerID as the collection owner. Every kbdb collection route is
// shaped /users/{userId}/..., so the owner being asked about is always known
// from the path before any repository query runs — callers scope the
// DynamoDB query (or a single-item fetch) to the returned set directly,
// rather than fetching everything and filtering per item after the fact.
//
//   - ownerID == caller's subject: all tiers (it's their own collection).
//   - no caller (anonymous): public only.
//   - caller present but ownerID differs: public + authenticated.
func ReadableVisibilities(ctx context.Context, ownerID string) []repository.Visibility {
	subject, _ := kbdbctx.UserID(ctx)
	if subject != "" && subject == ownerID {
		return []repository.Visibility{
			repository.VisibilityPublic,
			repository.VisibilityAuthenticated,
			repository.VisibilityPrivate,
		}
	}

	if subject == "" {
		return []repository.Visibility{repository.VisibilityPublic}
	}

	return []repository.Visibility{
		repository.VisibilityPublic,
		repository.VisibilityAuthenticated,
	}
}

// IsOwner reports whether the caller identified by ctx is ownerID. Used to
// gate writes, which are always owner-only regardless of visibility.
func IsOwner(ctx context.Context, ownerID string) bool {
	subject, _ := kbdbctx.UserID(ctx)
	return subject != "" && subject == ownerID
}
