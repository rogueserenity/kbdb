package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedKeycapSet PutItems a keycap set directly into DynamoDB, bypassing the
// API - for specs that need keycap set fixture data in place before
// exercising a different route. version is set to 0, matching what Create
// would have set it to - kit mutations condition their write on version
// matching what they read, so a seeded item without one wouldn't satisfy
// that condition the way attribute_not_exists would.
func SeedKeycapSet(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "GMK",
		"name":       "Laser",
		"visibility": visibility,
		"version":    0,
	})
}

// DeleteKeycapSet removes a keycap set seeded by SeedKeycapSet, or one
// created via the API during a spec.
func DeleteKeycapSet(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
