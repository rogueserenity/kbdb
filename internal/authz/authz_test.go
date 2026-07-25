package authz_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rogueserenity/kbdb/internal/authz"
	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

func TestReadableVisibilities(t *testing.T) {
	tests := []struct {
		name    string
		ownerID string
		caller  string // "" means no user ID set on the context at all
		want    []repository.Visibility
	}{
		{
			"owner requesting own collection",
			"alice", "alice",
			[]repository.Visibility{repository.VisibilityPublic, repository.VisibilityAuthenticated, repository.VisibilityPrivate},
		},
		{
			"anonymous requesting someone's collection",
			"alice", "",
			[]repository.Visibility{repository.VisibilityPublic},
		},
		{
			"other authenticated user requesting someone's collection",
			"alice", "bob",
			[]repository.Visibility{repository.VisibilityPublic, repository.VisibilityAuthenticated},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.caller != "" {
				ctx = kbdbctx.WithUserID(ctx, tt.caller)
			}

			assert.ElementsMatch(t, tt.want, authz.ReadableVisibilities(ctx, tt.ownerID))
		})
	}
}

func TestCanReadVisibility(t *testing.T) {
	tests := []struct {
		name       string
		ownerID    string
		visibility repository.Visibility
		caller     string
		want       bool
	}{
		{"owner reading own private item", "alice", repository.VisibilityPrivate, "alice", true},
		{"anonymous reading public item", "alice", repository.VisibilityPublic, "", true},
		{"anonymous reading authenticated item", "alice", repository.VisibilityAuthenticated, "", false},
		{"anonymous reading private item", "alice", repository.VisibilityPrivate, "", false},
		{"other user reading authenticated item", "alice", repository.VisibilityAuthenticated, "bob", true},
		{"other user reading private item", "alice", repository.VisibilityPrivate, "bob", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.caller != "" {
				ctx = kbdbctx.WithUserID(ctx, tt.caller)
			}

			assert.Equal(t, tt.want, authz.CanReadVisibility(ctx, tt.ownerID, tt.visibility))
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

			assert.Equal(t, tt.want, authz.IsOwner(ctx, tt.ownerID))
		})
	}
}
