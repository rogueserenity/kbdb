package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rogueserenity/kbdb/internal/authz"
	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// fakeOwned is a minimal authz.Owned implementation, kept local to this
// test so authz's tests don't depend on any one entity's struct shape.
type fakeOwned struct {
	ownerID    string
	visibility repository.Visibility
}

func (f fakeOwned) OwnerID() string                       { return f.ownerID }
func (f fakeOwned) VisibilityTier() repository.Visibility { return f.visibility }

func newFakeOwned(userID string, visibility repository.Visibility) fakeOwned {
	return fakeOwned{ownerID: userID, visibility: visibility}
}

func TestCanRead(t *testing.T) {
	tests := []struct {
		name       string
		ownerID    string
		visibility repository.Visibility
		caller     string // "" means no user ID set on the context at all
		want       bool
	}{
		{"owner reading own private item", "alice", repository.VisibilityPrivate, "alice", true},
		{"owner reading own public item", "alice", repository.VisibilityPublic, "alice", true},
		{"anonymous reading public item", "alice", repository.VisibilityPublic, "", true},
		{"anonymous reading authenticated item", "alice", repository.VisibilityAuthenticated, "", false},
		{"anonymous reading private item", "alice", repository.VisibilityPrivate, "", false},
		{"other user reading authenticated item", "alice", repository.VisibilityAuthenticated, "bob", true},
		{"other user reading public item", "alice", repository.VisibilityPublic, "bob", true},
		{"other user reading private item", "alice", repository.VisibilityPrivate, "bob", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.caller != "" {
				ctx = kbdbctx.WithUserID(ctx, tt.caller)
			}

			item := newFakeOwned(tt.ownerID, tt.visibility)

			assert.Equal(t, tt.want, authz.CanRead(ctx, item))
		})
	}
}

func TestIsOwner(t *testing.T) {
	tests := []struct {
		name    string
		ownerID string
		caller  string
		want    bool
	}{
		{"owner", "alice", "alice", true},
		{"other user", "alice", "bob", false},
		{"anonymous", "alice", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.caller != "" {
				ctx = kbdbctx.WithUserID(ctx, tt.caller)
			}

			item := newFakeOwned(tt.ownerID, repository.VisibilityPrivate)

			assert.Equal(t, tt.want, authz.IsOwner(ctx, item))
		})
	}
}
