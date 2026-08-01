package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedKeycapSet PutItems a keycap set directly into DynamoDB, bypassing the
// API - for specs that need keycap set fixture data in place before
// exercising a different route.
func SeedKeycapSet(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "GMK",
		"name":       "Laser",
		"visibility": visibility,
	})
}

// DeleteKeycapSet removes a keycap set seeded by SeedKeycapSet, or one
// created via the API during a spec.
func DeleteKeycapSet(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
