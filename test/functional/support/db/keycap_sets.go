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

// SeedKeycapSetWithKit is SeedKeycapSet, but includes a single kit with the
// given kitID in its Kits, so DeleteKit specs have something to delete.
func SeedKeycapSetWithKit(ctx context.Context, ownerID, id, kitID, visibility string) error {
	return SeedKeycapSetWithKits(ctx, ownerID, id, []string{kitID}, visibility)
}

// SeedKeycapSetWithKits is SeedKeycapSet, but includes one kit per given
// kitID in its Kits, so whole-set DeleteKeycapSet specs can exercise
// multiple kits referenced by different builds.
func SeedKeycapSetWithKits(ctx context.Context, ownerID, id string, kitIDs []string, visibility string) error {
	kits := make([]map[string]any, len(kitIDs))
	for i, kitID := range kitIDs {
		kits[i] = map[string]any{"kit_id": kitID, "name": "Base", "purchase": map[string]any{
			"vendor": "MechMarket",
			"price":  85.0,
		}}
	}

	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "GMK",
		"name":       "Laser",
		"kits":       kits,
		"visibility": visibility,
		"version":    0,
	})
}

// SeedKeycapSetWithPrimaryKit is SeedKeycapSetWithKit, but also sets
// primary_kit_id to kitID - there's no write path for it yet (see #247), so
// specs exercising the read side seed it directly.
func SeedKeycapSetWithPrimaryKit(ctx context.Context, ownerID, id, kitID, visibility string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id": ownerID,
		"id":      id,
		"brand":   "GMK",
		"name":    "Laser",
		"kits": []map[string]any{
			{"kit_id": kitID, "name": "Base", "purchase": map[string]any{
				"vendor": "MechMarket",
				"price":  85.0,
			}},
		},
		"primary_kit_id": kitID,
		"visibility":     visibility,
		"version":        0,
	})
}

// SeedKeycapSetWithDanglingPrimaryKit is SeedKeycapSet, but sets
// primary_kit_id to a kit id absent from Kits - simulates the primary kit
// having been deleted after being designated.
func SeedKeycapSetWithDanglingPrimaryKit(ctx context.Context, ownerID, id, visibility string) error {
	table := NewDynamoTable(ctx, support.KeycapSetTableName())
	return table.PutItem(ctx, map[string]any{
		"user_id":        ownerID,
		"id":             id,
		"brand":          "GMK",
		"name":           "Laser",
		"primary_kit_id": "no-longer-a-kit",
		"visibility":     visibility,
		"version":        0,
	})
}
