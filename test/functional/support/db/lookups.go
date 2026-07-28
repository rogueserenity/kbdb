package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedLookupCategory PutItems a lookup category directly into DynamoDB,
// bypassing the API - for specs that need lookup fixture data in place
// before exercising a different route. values is []any (not []string)
// since some categories store objects instead of plain strings - see
// model/lookup_seed.json.
func SeedLookupCategory(ctx context.Context, category string, values []any) error {
	table := NewDynamoTable(ctx, support.LookupTableName())
	return table.PutItem(ctx, map[string]any{
		"category": category,
		"values":   values,
	})
}

// DeleteLookupCategory removes a category seeded by SeedLookupCategory, or
// one created via the API during a spec. Idempotent - deleting a category
// that was never created is a harmless no-op.
func DeleteLookupCategory(ctx context.Context, category string) error {
	table := NewDynamoTable(ctx, support.LookupTableName())
	return table.DeleteItem(ctx, map[string]string{"category": category})
}
