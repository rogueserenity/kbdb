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
			ProjectionExpression: aws.String("category"),
			ExclusiveStartKey:    lastKey,
		})
		if err != nil {
			return nil, fmt.Errorf("scanning lookup table: %w", err)
		}

		var rows []struct {
			Category string `dynamodbav:"category"`
		}
		if err := attributevalue.UnmarshalListOfMaps(out.Items, &rows); err != nil {
			return nil, fmt.Errorf("unmarshalling lookup categories: %w", err)
		}
		for _, row := range rows {
			categories = append(categories, row.Category)
		}

		lastKey = out.LastEvaluatedKey
		if len(lastKey) == 0 {
			break
		}
	}

	sort.Strings(categories)

	return categories, nil
}

func (r *LookupRepository) GetCategory(ctx context.Context, category string) (*repository.Lookup, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"category": &types.AttributeValueMemberS{Value: category},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting lookup category %q: %w", category, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var lookup repository.Lookup
	if err := attributevalue.UnmarshalMap(out.Item, &lookup); err != nil {
		return nil, fmt.Errorf("unmarshalling lookup category %q: %w", category, err)
	}

	return &lookup, nil
}
