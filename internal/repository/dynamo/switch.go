package dynamo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
)

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
		Limit:                     queryLimit(limit),
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
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, repository.ErrAlreadyExists
		}
		return nil, fmt.Errorf("creating switch %q for owner %q: %w", sw.ID, sw.UserID, err)
	}

	return &sw, nil
}

// Update rewrites the caller's switch from the request body with a single
// UpdateItem: named attributes are SET (or REMOVEd when the body omitted an
// optional field), and image_path - which the body never carries - survives
// by not being named.
//
// The only ConditionExpression is attribute_exists(id): a full-body PUT is
// last-write-wins against a concurrent PUT, and the update must not
// resurrect a row a concurrent Delete just removed. A condition failure
// therefore means the row is gone; classifySwitchConflict re-reads with a
// consistent GetItem to tell ErrNotFound (404) from a lagging replica that
// still shows the row (ErrMutationConflict, 409).
func (r *SwitchRepository) Update(ctx context.Context, sw repository.Switch) (*repository.Switch, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating switch %q: %w", sw.ID, repository.ErrNoUserID)
	}

	update := expression.
		Set(expression.Name("brand"), expression.Value(sw.Brand)).
		Set(expression.Name("name"), expression.Value(sw.Name)).
		Set(expression.Name("type"), expression.Value(sw.Type)).
		Set(expression.Name("material"), expression.Value(sw.Material)).
		Set(expression.Name("force"), expression.Value(sw.Force)).
		Set(expression.Name("spring"), expression.Value(sw.Spring)).
		Set(expression.Name("purchase"), expression.Value(sw.Purchase)).
		Set(expression.Name("visibility"), expression.Value(sw.Visibility))
	update = setOrRemovePtr(update, "manufacturer", sw.Manufacturer)
	update = setOrRemovePtr(update, "pins", sw.Pins)
	update = setOrRemovePtr(update, "factory_lubed", sw.FactoryLubed)
	update = setOrRemovePtr(update, "notes", sw.Notes)

	expr, err := expression.NewBuilder().
		WithUpdate(update).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building switch update expression for switch %q: %w", sw.ID, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, sw.ID),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              types.ReturnValueAllNew,
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, r.classifySwitchConflict(ctx, ownerID, sw.ID)
		}
		return nil, fmt.Errorf("updating switch %q for owner %q: %w", sw.ID, ownerID, err)
	}

	var updated repository.Switch
	if err := attributevalue.UnmarshalMap(out.Attributes, &updated); err != nil {
		return nil, fmt.Errorf("unmarshalling updated switch %q for owner %q: %w", sw.ID, ownerID, err)
	}

	return &updated, nil
}

// classifySwitchConflict resolves an UpdateItem ConditionalCheckFailed on
// attribute_exists(id): a strongly-consistent GetItem that finds nothing
// means the row was deleted concurrently (ErrNotFound); a row still present
// means the failed condition raced a concurrent write that has since
// completed (ErrMutationConflict). ConsistentRead is required here - an
// eventually-consistent read against a lagging replica could still show a
// just-deleted row and misreport the 404 as a 409.
func (r *SwitchRepository) classifySwitchConflict(ctx context.Context, ownerID, id string) error {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      &r.tableName,
		Key:            switchKey(ownerID, id),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("classifying switch %q conflict for owner %q: %w", id, ownerID, err)
	}
	if len(out.Item) == 0 {
		return repository.ErrNotFound
	}
	return repository.ErrMutationConflict
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

// switchKey builds the base-table primary key for a switch.
func switchKey(ownerID, switchID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: ownerID},
		"id":      &types.AttributeValueMemberS{Value: switchID},
	}
}

// setOrRemovePtr adds a SET for an optional pointer field when it's
// populated, or a REMOVE when it's nil - so a PUT-style Update that omits an
// optional field clears the stored attribute, matching the whole-item
// replace semantics callers expect.
func setOrRemovePtr[T any](update expression.UpdateBuilder, name string, v *T) expression.UpdateBuilder {
	if v == nil {
		return update.Remove(expression.Name(name))
	}
	return update.Set(expression.Name(name), expression.Value(*v))
}

// SetImagePath implements repository.SwitchRepository. One UpdateItem SET
// under attribute_exists(id) - image_path isn't version-guarded, it's a
// single owner-scoped attribute with no read-derived value.
func (r *SwitchRepository) SetImagePath(ctx context.Context, id string, key repository.SwitchImageKey) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("setting image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	expr, err := expression.NewBuilder().
		WithUpdate(expression.Set(expression.Name("image_path"), expression.Value(key))).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return fmt.Errorf("building switch image path update for switch %q: %w", id, err)
	}

	_, err = r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return repository.ErrNotFound
		}
		return fmt.Errorf("setting image path for switch %q owner %q: %w", id, ownerID, err)
	}

	return nil
}

// ClearImagePath implements repository.SwitchRepository. One UpdateItem
// REMOVE under attribute_exists(id), with ReturnValues ALL_OLD: the
// pre-image's image_path (or its absence) is the "what was cleared" answer,
// so no sentinel and no read are needed to report "nothing was set".
func (r *SwitchRepository) ClearImagePath(ctx context.Context, id string) (*repository.SwitchImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("clearing image path for switch %q: %w", id, repository.ErrNoUserID)
	}

	expr, err := expression.NewBuilder().
		WithUpdate(expression.Remove(expression.Name("image_path"))).
		WithCondition(expression.AttributeExists(expression.Name("id"))).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building switch image path clear for switch %q: %w", id, err)
	}

	out, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 &r.tableName,
		Key:                       switchKey(ownerID, id),
		UpdateExpression:          expr.Update(),
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ReturnValues:              types.ReturnValueAllOld,
	})
	if err != nil {
		if _, ok := errors.AsType[*types.ConditionalCheckFailedException](err); ok {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("clearing image path for switch %q owner %q: %w", id, ownerID, err)
	}

	old := struct {
		ImagePath *repository.SwitchImageKey `dynamodbav:"image_path"`
	}{}
	if err := attributevalue.UnmarshalMap(out.Attributes, &old); err != nil {
		return nil, fmt.Errorf("unmarshalling cleared switch %q image path for owner %q: %w", id, ownerID, err)
	}
	if old.ImagePath == nil {
		return nil, nil //nolint:nilnil // no image already set is a valid, expected result
	}

	return old.ImagePath, nil
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

// encodeCursor is decodeCursor's inverse. Every LastEvaluatedKey paginated
// here is string-valued (base-table user_id/id and the directory GSI
// keys), so a plain map[string]string round-trips it without
// attributevalue's cursor-unfriendly type-tagged encoding.
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
