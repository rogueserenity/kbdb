package dynamo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// errSwitchImageAlreadyAbsent signals ClearImagePath's mutateSwitch closure
// found no ImagePath set - ClearImagePath treats this as success, not an
// error.
var errSwitchImageAlreadyAbsent = errors.New("image already absent from switch")

// SwitchRepository is the DynamoDB-backed repository.SwitchRepository.
type SwitchRepository struct {
	client    dynamoAPI
	tableName string
}

var _ repository.SwitchRepository = (*SwitchRepository)(nil)

// NewSwitchRepository returns a SwitchRepository backed by client.
func NewSwitchRepository(client *dynamodb.Client, tableName string) *SwitchRepository {
	return &SwitchRepository{client: client, tableName: tableName}
}

// List implements repository.SwitchRepository.
func (r *SwitchRepository) List(
	ctx context.Context,
	ownerID string,
	visibilities []repository.Visibility,
	limit int,
	cursor string,
) ([]repository.Switch, string, error) {
	// No visibility tier is readable, so nothing can match - short-circuit
	// rather than build a Query with an empty IN(...) filter (which the
	// expression builder below would panic constructing).
	if len(visibilities) == 0 {
		return []repository.Switch{}, "", nil
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
		return nil, "", fmt.Errorf("building switch list expression: %w", err)
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
		return nil, "", fmt.Errorf("querying switches for owner %q: %w", ownerID, err)
	}

	switches := []repository.Switch{}
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &switches); err != nil {
		return nil, "", fmt.Errorf("unmarshalling switches for owner %q: %w", ownerID, err)
	}

	nextCursor, err := encodeCursor(out.LastEvaluatedKey)
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return switches, nextCursor, nil
}

// Get implements repository.SwitchRepository.
func (r *SwitchRepository) Get(ctx context.Context, ownerID, id string) (*repository.Switch, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting switch %q for owner %q: %w", id, ownerID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var sw repository.Switch
	if err := attributevalue.UnmarshalMap(out.Item, &sw); err != nil {
		return nil, fmt.Errorf("unmarshalling switch %q for owner %q: %w", id, ownerID, err)
	}

	return &sw, nil
}

// Create implements repository.SwitchRepository.
func (r *SwitchRepository) Create(ctx context.Context, sw repository.Switch) (*repository.Switch, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating switch %q: %w", sw.ID, repository.ErrNoUserID)
	}
	sw.UserID = ownerID

	item, err := attributevalue.MarshalMap(sw)
	if err != nil {
		return nil, fmt.Errorf("marshalling switch %q for owner %q: %w", sw.ID, sw.UserID, err)
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
		return nil, fmt.Errorf("creating switch %q for owner %q: %w", sw.ID, sw.UserID, err)
	}

	return &sw, nil
}

// Update goes through mutateSwitch rather than a naive whole-item PutItem:
// sw (built from the request body) never has ImagePath or Version set, so
// overwriting the stored item wholesale would wipe the image and desync
// Version from mutateSwitch's CAS loop.
func (r *SwitchRepository) Update(ctx context.Context, sw repository.Switch) (*repository.Switch, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating switch %q: %w", sw.ID, repository.ErrNoUserID)
	}

	updated, err := r.mutateSwitch(ctx, ownerID, sw.ID, func(existing *repository.Switch) error {
		existing.Brand = sw.Brand
		existing.Manufacturer = sw.Manufacturer
		existing.Name = sw.Name
		existing.Type = sw.Type
		existing.Pins = sw.Pins
		existing.FactoryLubed = sw.FactoryLubed
		existing.Material = sw.Material
		existing.Force = sw.Force
		existing.Spring = sw.Spring
		existing.Purchase = sw.Purchase
		existing.Notes = sw.Notes
		existing.Visibility = sw.Visibility
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("updating switch %q for owner %q: %w", sw.ID, ownerID, err)
	}

	return updated, nil
}

// Delete implements repository.SwitchRepository. Idempotent: a nonexistent
// id is not an error.
func (r *SwitchRepository) Delete(ctx context.Context, id string) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("deleting switch %q: %w", id, repository.ErrNoUserID)
	}

	_, err := r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &r.tableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
			"id":      &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		return fmt.Errorf("deleting switch %q for owner %q: %w", id, ownerID, err)
	}

	return nil
}

const maxSwitchMutationAttempts = 3

// mutateSwitch is a hand-rolled Version-based CAS retry loop, mirroring
// [(*KeycapSetRepository).mutateSet]/[(*KeyboardRepository).mutateKeyboard]:
// DynamoDB has no built-in optimistic-locking primitive the Go SDK exposes,
// so a mutation that must coexist with Update's own whole-item write reads,
// mutates, and conditionally rewrites the whole item under a Version guard.
func (r *SwitchRepository) mutateSwitch(
	ctx context.Context,
	ownerID, switchID string,
	mutate func(sw *repository.Switch) error,
) (*repository.Switch, error) {
	for range maxSwitchMutationAttempts {
		sw, err := r.Get(ctx, ownerID, switchID)
		if err != nil {
			return nil, err
		}

		if err := mutate(sw); err != nil {
			return nil, err
		}

		expectedVersion := sw.Version
		sw.Version++
		sw.UserID = ownerID

		item, err := attributevalue.MarshalMap(*sw)
		if err != nil {
			return nil, fmt.Errorf("marshalling switch %q for owner %q: %w", switchID, ownerID, err)
		}

		expr, err := expression.NewBuilder().
			WithCondition(expression.Name("version").Equal(expression.Value(expectedVersion))).
			Build()
		if err != nil {
			return nil, fmt.Errorf("building switch mutation condition for switch %q: %w", switchID, err)
		}

		_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
			TableName:                 &r.tableName,
			Item:                      item,
			ConditionExpression:       expr.Condition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		})
		if err == nil {
			return sw, nil
		}

		var condErr *types.ConditionalCheckFailedException
		if !errors.As(err, &condErr) {
			return nil, fmt.Errorf("mutating switch %q owner %q: %w", switchID, ownerID, err)
		}
		// Lost the CAS race - another writer updated Version first. Loop
		// and retry from a fresh Get.
		log.FromContext(ctx).Warn("switch CAS retry", log.SwitchID, switchID, "attempted_version", expectedVersion)
	}

	return nil, fmt.Errorf("mutating switch %q owner %q: %w", switchID, ownerID, repository.ErrMutationConflict)
}

// SetImagePath implements repository.SwitchRepository.
func (r *SwitchRepository) SetImagePath(ctx context.Context, id string, key repository.SwitchImageKey) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("setting image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	_, err := r.mutateSwitch(ctx, ownerID, id, func(sw *repository.Switch) error {
		sw.ImagePath = &key
		return nil
	})
	return err
}

// ClearImagePath implements repository.SwitchRepository.
func (r *SwitchRepository) ClearImagePath(ctx context.Context, id string) (*repository.SwitchImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("clearing image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	var cleared *repository.SwitchImageKey
	_, err := r.mutateSwitch(ctx, ownerID, id, func(sw *repository.Switch) error {
		if sw.ImagePath == nil {
			return errSwitchImageAlreadyAbsent
		}
		cleared = sw.ImagePath
		sw.ImagePath = nil
		return nil
	})
	if errors.Is(err, errSwitchImageAlreadyAbsent) {
		return nil, nil //nolint:nilnil // no image already set is a valid, expected result
	}
	if err != nil {
		return nil, err
	}

	return cleared, nil
}

// decodeCursor reverses encodeCursor, returning nil (no key) for an empty
// cursor.
func decodeCursor(cursor string) (map[string]types.AttributeValue, error) {
	if cursor == "" {
		return nil, nil //nolint:nilnil // no key is a valid, expected result
	}

	raw, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	var key map[string]string
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, fmt.Errorf("invalid cursor contents: %w", err)
	}

	out := make(map[string]types.AttributeValue, len(key))
	for k, v := range key {
		out[k] = &types.AttributeValueMemberS{Value: v}
	}

	return out, nil
}

// encodeCursor is decodeCursor's inverse. DynamoDB's LastEvaluatedKey for
// this table is always string-valued (user_id, id), so a plain
// map[string]string round-trips it without needing attributevalue's fuller
// (and cursor-unfriendly) type-tagged encoding.
func encodeCursor(key map[string]types.AttributeValue) (string, error) {
	if len(key) == 0 {
		return "", nil
	}

	plain := make(map[string]string, len(key))
	for k, v := range key {
		s, ok := v.(*types.AttributeValueMemberS)
		if !ok {
			return "", fmt.Errorf("unexpected non-string attribute %q in last evaluated key", k)
		}
		plain[k] = s.Value
	}

	raw, err := json.Marshal(plain)
	if err != nil {
		return "", fmt.Errorf("marshalling cursor: %w", err)
	}

	return base64.URLEncoding.EncodeToString(raw), nil
}
