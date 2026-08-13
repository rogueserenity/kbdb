package db

import (
	"context"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// DeleteBuild removes a build created via the API during a spec.
func DeleteBuild(ctx context.Context, ownerID, id string) error {
	table := NewDynamoTable(ctx, support.BuildTableName())
	return table.DeleteItem(ctx, map[string]string{"user_id": ownerID, "id": id})
}
