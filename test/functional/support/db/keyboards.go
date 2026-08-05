package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedKeyboard PutItems a keyboard directly into DynamoDB, bypassing the
// API - for specs that need keyboard fixture data in place before
// exercising a different route.
//
// The nested design/pcb/purchase groups are populated so reads exercise
// their real DynamoDB round trip. Seeding only the top-level fields would
// leave a dynamodbav tag mismatch on a nested group invisible to every
// spec, since the mappers' own tests construct Go structs directly and
// never touch DynamoDB.
func SeedKeyboard(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.KeyboardTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "Keychron",
		"name":       "Q1",
		"size":       "60%",
		"visibility": visibility,
		"design": map[string]any{
			"top_case": map[string]any{"material": "Aluminum", "color": "Black"},
			"plates":   []string{"Brass"},
		},
		"pcb": map[string]any{"firmware": "QMK/VIA"},
		"purchase": map[string]any{
			"vendor":       "Amazon",
			"order_status": "Delivered",
		},
	})
}

// DeleteKeyboard removes a keyboard seeded by SeedKeyboard, or one created
// via the API during a spec.
func DeleteKeyboard(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.KeyboardTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
