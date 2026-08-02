// Package dynamo holds DynamoDB-backed implementations of the interfaces
// declared in internal/repository, one file per entity.
package dynamo

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// errNoUserID guards every Create/Update/Delete against a silently ignored
// kbdbctx.UserID ok - an empty-string partition key would otherwise write or
// delete against user_id="" instead of erroring. Create/Update's
// ConditionExpression only guards an id collision, not a missing user_id,
// so it's not a substitute for this check.
var errNoUserID = errors.New("no user id in context")

// errKitMissingAfterAdd guards AddKit's return value: it looks the just-
// added kit back up by KitID in the set mutateSet actually persisted,
// rather than assuming a position (e.g. "the last element") or returning
// the caller's pre-write input - this is that lookup failing, which would
// mean mutateSet's returned state doesn't actually contain the kit it just
// appended, a stronger contract violation than a plain CAS failure.
var errKitMissingAfterAdd = errors.New("kit not found in set after AddKit")

// dynamoAPI is the subset of *dynamodb.Client's methods used by the
// repository implementations in this package.
type dynamoAPI interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
}
