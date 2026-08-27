package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
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

// Create implements repository.ProfileRepository. It writes the profile
// item (conditional on the user not already having one) and the
// { username -> user_id } claim item (conditional on the username being
// unclaimed) in a single TransactWriteItems, so a username is never
// half-claimed. The two conditional failures are told apart by which
// cancellation reason fired: index 0 is the profile Put (ErrAlreadyExists),
// index 1 is the claim Put (ErrUsernameTaken).
func (r *ProfileRepository) Create(ctx context.Context, p repository.Profile) (*repository.Profile, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating profile: %w", repository.ErrNoUserID)
	}
	p.StytchUserID = ownerID
	p.Version = 0
	setProfileDirectoryKeys(&p)

	profileItem, err := attributevalue.MarshalMap(p)
	if err != nil {
		return nil, fmt.Errorf("marshalling profile for user %q: %w", ownerID, err)
	}

	claimItem, err := attributevalue.MarshalMap(struct {
		Username string `dynamodbav:"username"`
		UserID   string `dynamodbav:"user_id"`
	}{Username: p.Username, UserID: ownerID})
	if err != nil {
		return nil, fmt.Errorf("marshalling username claim for user %q: %w", ownerID, err)
	}

	_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: []types.TransactWriteItem{
			{Put: &types.Put{
				TableName:           &r.profileTableName,
				Item:                profileItem,
				ConditionExpression: aws.String("attribute_not_exists(user_id)"),
			}},
			{Put: &types.Put{
				TableName:           &r.usernameTableName,
				Item:                claimItem,
				ConditionExpression: aws.String("attribute_not_exists(username)"),
			}},
		},
	})
	if err != nil {
		if mapped := mapProfileCreateConflict(err); mapped != nil {
			return nil, mapped
		}
		return nil, fmt.Errorf("creating profile for user %q: %w", ownerID, err)
	}

	return &p, nil
}

// setProfileDirectoryKeys derives the sparse-GSI discriminators from p's
// Discoverable / DiscordUsername fields. Left nil (and so omitted from the
// item) whenever the profile shouldn't be in that index.
func setProfileDirectoryKeys(p *repository.Profile) {
	p.DiscoverablePK = nil
	p.DiscordPK = nil
	p.DiscordUsernameLC = nil

	if !p.Discoverable {
		return
	}

	p.DiscoverablePK = aws.String("1")

	if p.DiscordUsername != nil {
		p.DiscordPK = aws.String("1")
		lc := strings.ToLower(*p.DiscordUsername)
		p.DiscordUsernameLC = &lc
	}
}

// mapProfileCreateConflict maps a TransactWriteItems ConditionExpression
// failure to the right sentinel: reason 0 (the profile Put) -> the user
// already has a profile; reason 1 (the claim Put) -> the username is taken.
// Returns nil if err isn't a conditional-check cancellation.
func mapProfileCreateConflict(err error) error {
	txErr, ok := errors.AsType[*types.TransactionCanceledException](err)
	if !ok {
		return nil
	}

	failed := func(i int) bool {
		return i < len(txErr.CancellationReasons) &&
			txErr.CancellationReasons[i].Code != nil &&
			*txErr.CancellationReasons[i].Code == "ConditionalCheckFailed"
	}

	switch {
	case failed(0):
		return repository.ErrAlreadyExists
	case failed(1):
		return repository.ErrUsernameTaken
	default:
		return nil
	}
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
