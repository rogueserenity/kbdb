package db

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedKeyboard PutItems a keyboard directly into DynamoDB, bypassing the
// API - for specs that need keyboard fixture data in place before
// exercising a different route. version is set to 0, matching what Create
// would have set it to - Update/AddImage/DeleteImage condition their write
// on version via a CAS loop, which fails on a fixture item missing the
// attribute entirely (attribute_not_exists != 0), not just a genuine
// version mismatch.
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
			"price":        329.99,
			"order_status": "Delivered",
		},
	})
}

// SeedKeyboardWithImage is [SeedKeyboard] plus a single Images entry, whose
// path doesn't need a real S3 object behind it - presigning a GET URL
// doesn't check the object exists, only specs that fetch the URL's content
// would need that. Images is a map keyed by image id, each entry carrying a
// seq ordering key (see repository.KeyboardImageEntry).
func SeedKeyboardWithImage(ctx context.Context, ownerID, id, imageID, visibility string) error {
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
			"price":        329.99,
			"order_status": "Delivered",
		},
		"images": map[string]any{
			imageID: map[string]any{
				"path": fmt.Sprintf("keyboards/%s/%s/images/%s", ownerID, id, imageID),
				"seq":  0,
			},
		},
	})
}

// DeleteKeyboard removes a keyboard seeded by SeedKeyboard, or one created
// via the API during a spec.
func DeleteKeyboard(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.KeyboardTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
