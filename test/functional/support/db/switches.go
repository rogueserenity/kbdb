package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedSwitch PutItems a switch directly into DynamoDB, bypassing the API -
// for specs that need switch fixture data in place before exercising a
// different route.
func SeedSwitch(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "Gateron",
		"name":       "Yellow",
		"type":       "Linear",
		"visibility": visibility,
	})
}

// DeleteSwitch removes a switch seeded by SeedSwitch, or one created via the
// API during a spec.
func DeleteSwitch(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.SwitchTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
