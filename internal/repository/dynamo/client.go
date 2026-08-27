package dynamo

import (
	"context"
	"errors"
	"math"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// queryLimit clamps a caller-supplied page size to [1, math.MaxInt32] and
// returns it as the *int32 a QueryInput.Limit wants. The two returns above
// the final conversion keep limit provably in range at the int32() cast -
// both for correctness and so static analysis can see the bound.
func queryLimit(limit int) *int32 {
	if limit < 1 {
		return aws.Int32(1)
	}
	if limit > math.MaxInt32 {
		return aws.Int32(math.MaxInt32)
	}
	return aws.Int32(int32(limit))
}

// errKitMissingAfterAdd means AddKit's just-appended kit isn't present by
// KitID in the set mutateSet returned - should be unreachable in practice.
var errKitMissingAfterAdd = errors.New("kit not found in set after AddKit")

// errKitAlreadyAbsent signals DeleteKit's mutateSet closure found no
// matching kit - DeleteKit treats this as success, not an error.
var errKitAlreadyAbsent = errors.New("kit already absent from set")

// errKitImageAlreadyAbsent signals ClearKitImagePath's mutateSet closure
// found the kit with no ImagePath set - ClearKitImagePath treats this as
// success, not an error.
var errKitImageAlreadyAbsent = errors.New("kit image already absent")

// dynamoAPI is the subset of *dynamodb.Client's methods used by the
// repository implementations in this package.
type dynamoAPI interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}
