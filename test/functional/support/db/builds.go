package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// keyboardRefMarkerSortKey mirrors the marker item sort key format
// [github.com/rogueserenity/kbdb/internal/repository/dynamo] writes for a
// keyboard reference, so seeded builds are discoverable via
// [github.com/rogueserenity/kbdb/internal/repository.BuildRepository.FindBuildsReferencingKeyboard]
// without needing a real Create call.
func keyboardRefMarkerSortKey(keyboardID, buildID string) string {
	return fmt.Sprintf("REF#keyboard#%s#%s", keyboardID, buildID)
}

// SeedBuild writes directly to the table, bypassing buildrefs' existence
// check - keyboardID isn't validated against a real keyboard.
func SeedBuild(ctx context.Context, ownerID, id, keyboardID, visibility string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())
	if err := table.PutItem(ctx, map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"keyboard":   keyboardID,
		"visibility": visibility,
		"version":    0,
	}); err != nil {
		return err
	}

	return table.PutItem(ctx, map[string]any{
		"user_id":   ownerID,
		"id":        keyboardRefMarkerSortKey(keyboardID, id),
		"item_type": "build_ref_marker",
		"ref_type":  "keyboard",
		"ref_id":    keyboardID,
		"build_id":  id,
	})
}

// DeleteBuild removes the build with id, and its keyboard reverse-reference
// marker, from the table. Both deletes are attempted even if the first
// fails, so a transient failure on one doesn't orphan the other in the
// shared, long-lived table CI deploys per PR (see .github/workflows/ci.yml
// - that stack persists across every push until the PR closes, so leftover
// rows aren't cleaned up by a fresh-container teardown the way a local
// LocalStack run's would be).
func DeleteBuild(ctx context.Context, ownerID, id, keyboardID string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())

	buildErr := table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
	markerErr := table.DeleteItem(ctx, map[string]string{
		"user_id": ownerID,
		"id":      keyboardRefMarkerSortKey(keyboardID, id),
	})

	return errors.Join(buildErr, markerErr)
}
