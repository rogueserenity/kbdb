package dynamo

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type KeycapSetRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.KeycapSetRepository = (*KeycapSetRepository)(nil)

func NewKeycapSetRepository(client *dynamodb.Client, tableName string) *KeycapSetRepository {
	return &KeycapSetRepository{client: client, tableName: tableName}
}

func (r *KeycapSetRepository) List(
	ctx context.Context,
	ownerID string,
	visibilities []repository.Visibility,
	limit int,
	cursor string,
) ([]repository.KeycapSet, string, error) {
	// No visibility tier is readable, so nothing can match - short-circuit
	// rather than build a Query with an empty IN(...) filter (which the
	// expression builder below would panic constructing).
	if len(visibilities) == 0 {
		return []repository.KeycapSet{}, "", nil
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
		return nil, "", fmt.Errorf("building keycap set list expression: %w", err)
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
		return nil, "", fmt.Errorf("querying keycap sets for owner %q: %w", ownerID, err)
	}

	sets := []repository.KeycapSet{}
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &sets); err != nil {
		return nil, "", fmt.Errorf("unmarshalling keycap sets for owner %q: %w", ownerID, err)
	}

	nextCursor, err := encodeCursor(out.LastEvaluatedKey)
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return sets, nextCursor, nil
}

func (r *KeycapSetRepository) Get(ctx context.Context, ownerID, id string) (*repository.KeycapSet, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting keycap set %q for owner %q: %w", id, ownerID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var ks repository.KeycapSet
	if err := attributevalue.UnmarshalMap(out.Item, &ks); err != nil {
		return nil, fmt.Errorf("unmarshalling keycap set %q for owner %q: %w", id, ownerID, err)
	}

	return &ks, nil
}

func (r *KeycapSetRepository) Create(ctx context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating keycap set %q: %w", ks.ID, errNoUserID)
	}
	ks.UserID = ownerID

	item, err := attributevalue.MarshalMap(ks)
	if err != nil {
		return nil, fmt.Errorf("marshalling keycap set %q for owner %q: %w", ks.ID, ks.UserID, err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &r.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("creating keycap set %q for owner %q: %w", ks.ID, ks.UserID, err)
	}

	return &ks, nil
}

func (r *KeycapSetRepository) Update(ctx context.Context, ks repository.KeycapSet) (*repository.KeycapSet, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating keycap set %q: %w", ks.ID, errNoUserID)
	}
	ks.UserID = ownerID

	item, err := attributevalue.MarshalMap(ks)
	if err != nil {
		return nil, fmt.Errorf("marshalling keycap set %q for owner %q: %w", ks.ID, ks.UserID, err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &r.tableName,
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(id)"),
	})
	if err != nil {
		var condErr *types.ConditionalCheckFailedException
		if errors.As(err, &condErr) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("updating keycap set %q for owner %q: %w", ks.ID, ks.UserID, err)
	}

	return &ks, nil
}

func (r *KeycapSetRepository) Delete(ctx context.Context, id string) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("deleting keycap set %q: %w", id, errNoUserID)
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return fmt.Errorf("deleting keycap set %q for owner %q: %w", id, ownerID, err)
	}

	return nil
}

const maxKitMutationAttempts = 3

// mutateKits performs a bounded read-modify-write of setID's Kits slice
// under a Version-based CAS loop: Get the set, run mutate against the
// in-memory struct, increment Version, and PutItem with a
// ConditionExpression requiring the version to still match what was read.
// On a CAS miss (another writer won the race), re-Get and retry. This is
// necessary because DynamoDB's Go SDK has no built-in optimistic-locking
// primitive (unlike DynamoDBMapper on the Java SDK) and kit fields have no
// server-side way to be addressed by kit_id within a List attribute -
// mutate must operate on the whole decoded slice in Go.
func (r *KeycapSetRepository) mutateKits(
	ctx context.Context,
	ownerID, setID string,
	mutate func(ks *repository.KeycapSet) error,
) (*repository.KeycapSet, error) {
	for range maxKitMutationAttempts {
		ks, err := r.Get(ctx, ownerID, setID)
		if err != nil {
			return nil, err
		}

		if err := mutate(ks); err != nil {
			return nil, err
		}

		expectedVersion := ks.Version
		ks.Version++

		item, err := attributevalue.MarshalMap(*ks)
		if err != nil {
			return nil, fmt.Errorf("marshalling keycap set %q for owner %q: %w", setID, ownerID, err)
		}

		expr, err := expression.NewBuilder().
			WithCondition(expression.Name("version").Equal(expression.Value(expectedVersion))).
			Build()
		if err != nil {
			return nil, fmt.Errorf("building kit mutation condition for set %q: %w", setID, err)
		}

		_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:                 &r.tableName,
			Item:                      item,
			ConditionExpression:       expr.Condition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		})
		if err == nil {
			return ks, nil
		}

		var condErr *types.ConditionalCheckFailedException
		if !errors.As(err, &condErr) {
			return nil, fmt.Errorf("mutating kits for set %q owner %q: %w", setID, ownerID, err)
		}
		// Lost the CAS race - another writer updated Version first. Loop
		// and retry from a fresh Get.
	}

	return nil, fmt.Errorf("mutating kits for set %q owner %q: %w", setID, ownerID, errKitMutationExhausted)
}

func (r *KeycapSetRepository) AddKit(ctx context.Context, setID string, kit repository.KeycapKit) (*repository.KeycapKit, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("adding kit to keycap set %q: %w", setID, errNoUserID)
	}

	_, err := r.mutateKits(ctx, ownerID, setID, func(ks *repository.KeycapSet) error {
		ks.Kits = append(ks.Kits, kit)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &kit, nil
}
