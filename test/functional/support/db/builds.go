package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedBuild inserts a build directly into the builds table, bypassing the
// API/handlers layer - used by specs that need a build to exist as a
// precondition (e.g. get) without exercising create in the same spec.
// keyboard is a fixture id; the seeded build doesn't itself validate that a
// keyboard with that id exists, unlike a real create request would.
func SeedBuild(ctx context.Context, ownerID, id, keyboardID, visibility string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"keyboard":   keyboardID,
		"visibility": visibility,
		"version":    0,
	})
}

// DeleteBuild removes a build created via the API during a spec, or one
// seeded by SeedBuild.
func DeleteBuild(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
