package dynamo

import (
	"context"
	"fmt"
	"math"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type KeyboardRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.KeyboardRepository = (*KeyboardRepository)(nil)

func NewKeyboardRepository(client *dynamodb.Client, tableName string) *KeyboardRepository {
	return &KeyboardRepository{client: client, tableName: tableName}
}

func (r *KeyboardRepository) List(
	ctx context.Context,
	ownerID string,
	visibilities []repository.Visibility,
	limit int,
	cursor string,
) ([]repository.Keyboard, string, error) {
	// No visibility tier is readable, so nothing can match - short-circuit
	// rather than build a Query with an empty IN(...) filter (which the
	// expression builder below would panic constructing).
	if len(visibilities) == 0 {
		return []repository.Keyboard{}, "", nil
	}

	startKey, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", fmt.Errorf("decoding cursor: %w", err)
	}

	// limit is validated by the handler against api/openapi.yaml's Limit
	// parameter (1-100) before reaching here; List's exported signature
	// accepts any int, so clamp defensively rather than trust that.
	if limit < 1 {
		limit = 1
	} else if limit > math.MaxInt32 {
		limit = math.MaxInt32
	}

	visValues := make([]expression.OperandBuilder, len(visibilities))
	for i, v := range visibilities {
		visValues[i] = expression.Value(v)
	}

	builder := expression.NewBuilder().
		WithKeyCondition(expression.Key("user_id").Equal(expression.Value(ownerID))).
		WithFilter(expression.Name("visibility").In(visValues[0], visValues[1:]...))

	expr, err := builder.Build()
	if err != nil {
		return nil, "", fmt.Errorf("building keyboard list expression: %w", err)
	}

	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 &r.tableName,
		KeyConditionExpression:    expr.KeyCondition(),
		FilterExpression:          expr.Filter(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ExclusiveStartKey:         startKey,
		Limit:                     aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, "", fmt.Errorf("querying keyboards for owner %q: %w", ownerID, err)
	}

	keyboards := []repository.Keyboard{}
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &keyboards); err != nil {
		return nil, "", fmt.Errorf("unmarshalling keyboards for owner %q: %w", ownerID, err)
	}

	nextCursor, err := encodeCursor(out.LastEvaluatedKey)
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return keyboards, nextCursor, nil
}
