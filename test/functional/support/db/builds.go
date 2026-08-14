package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedBuild writes directly to the table, bypassing buildrefs' existence
// check - keyboardID isn't validated against a real keyboard.
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

// DeleteBuild removes the build with id from the table.
func DeleteBuild(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
