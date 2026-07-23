package dynamo

import (
	"context"
	"errors"
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

func (r *LookupRepository) ReplaceCategory(ctx context.Context, category string, values []any) (*repository.Lookup, error) {
	return r.putCategory(ctx, "replacing", category, values, "attribute_exists(category)", repository.ErrNotFound)
}

func (r *LookupRepository) DeleteCategory(ctx context.Context, category string) error {
	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"category": &types.AttributeValueMemberS{Value: category},
		},
	})
	if err != nil {
		return fmt.Errorf("deleting lookup category %q: %w", category, err)
	}

	return nil
}

func (r *LookupRepository) CreateCategory(ctx context.Context, category string, values []any) (*repository.Lookup, error) {
	return r.putCategory(ctx, "creating", category, values, "attribute_not_exists(category)", repository.ErrAlreadyExists)
}

// putCategory backs CreateCategory and ReplaceCategory, which differ only
// in the precondition (category must not/must already exist) and the
// sentinel error a failed condition maps to. verb ("creating"/"replacing")
// keeps the two callers' wrapped errors distinguishable.
func (r *LookupRepository) putCategory(ctx context.Context, verb, category string, values []any, condition string, conditionFailedErr error) (*repository.Lookup, error) {
	lookup := repository.Lookup{Category: category, Values: values}

	item, err := attributevalue.MarshalMap(lookup)
	if err != nil {
		return nil, fmt.Errorf("marshalling lookup category %q: %w", category, err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &r.tableName,
		Item:                item,
		ConditionExpression: aws.String(condition),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return nil, conditionFailedErr
		}
		return nil, fmt.Errorf("%s lookup category %q: %w", verb, category, err)
	}

	return &lookup, nil
}
