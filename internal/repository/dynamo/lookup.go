package dynamo

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rogueserenity/kbdb/internal/repository"
)

type LookupRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.LookupRepository = (*LookupRepository)(nil)

func NewLookupRepository(client *dynamodb.Client, tableName string) *LookupRepository {
	return &LookupRepository{client: client, tableName: tableName}
}

func (r *LookupRepository) ListCategories(ctx context.Context) ([]string, error) {
	categories := []string{}

	var lastKey map[string]types.AttributeValue
	for {
		out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
			TableName:            &r.tableName,
			ProjectionExpression: aws.String("PK"),
			ExclusiveStartKey:    lastKey,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning lookup table: %w", err)
		}

		var rows []struct {
			PK string `dynamodbav:"PK"`
		}
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &rows); err != nil {
			return nil, fmt.Errorf("unmarshalling lookup categories: %w", err)
		}
		for _, row := range rows {
			categories = append(categories, row.PK)
		}

		lastKey = out.LastEvaluatedKey
		if len(lastKey) == 0 {
			break
		}
	}

	sort.Strings(categories)

	return categories, nil
}
