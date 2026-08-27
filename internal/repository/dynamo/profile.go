package dynamo

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileRepository is the DynamoDB-backed repository.ProfileRepository.
// It uses two tables: profileTableName holds the profile items (partitioned
// by user_id, no sort key - one profile per user), and usernameTableName
// holds { username -> user_id } claim items that enforce username
// uniqueness (a GSI can't). Later issues add the write methods that keep
// the two in sync via TransactWriteItems.
type ProfileRepository struct {
	client            dynamoAPI
	profileTableName  string
	usernameTableName string
}

var _ repository.ProfileRepository = (*ProfileRepository)(nil)

// NewProfileRepository returns a ProfileRepository backed by client.
func NewProfileRepository(client *dynamodb.Client, profileTableName, usernameTableName string) *ProfileRepository {
	return &ProfileRepository{
		client:            client,
		profileTableName:  profileTableName,
		usernameTableName: usernameTableName,
	}
}

// Get implements repository.ProfileRepository.
func (r *ProfileRepository) Get(ctx context.Context, stytchUserID string) (*repository.Profile, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.profileTableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: stytchUserID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting profile for user %q: %w", stytchUserID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var p repository.Profile
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return nil, fmt.Errorf("unmarshalling profile for user %q: %w", stytchUserID, err)
	}

	return &p, nil
}

// ResolveUsername implements repository.ProfileRepository. It reads the
// { username -> user_id } claim item from usernameTableName; ErrNotFound
// means no profile has claimed that username.
func (r *ProfileRepository) ResolveUsername(ctx context.Context, username string) (string, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.usernameTableName,
		Key: map[string]types.AttributeValue{
			"username": &types.AttributeValueMemberS{Value: username},
		},
	})
	if err != nil {
		return "", fmt.Errorf("resolving username %q: %w", username, err)
	}

	if len(out.Item) == 0 {
		return "", repository.ErrNotFound
	}

	claim := struct {
		UserID string `dynamodbav:"user_id"`
	}{}
	if err := attributevalue.UnmarshalMap(out.Item, &claim); err != nil {
		return "", fmt.Errorf("unmarshalling username claim %q: %w", username, err)
	}
	if claim.UserID == "" {
		return "", fmt.Errorf("username claim %q has no user_id", username)
	}

	return claim.UserID, nil
}
