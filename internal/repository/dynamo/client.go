// Package dynamo holds DynamoDB-backed implementations of the interfaces
// declared in internal/repository, one file per entity.
package dynamo

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// dynamoAPI is the subset of *dynamodb.Client's methods used by the
// repository implementations in this package.
type dynamoAPI interface {
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}
