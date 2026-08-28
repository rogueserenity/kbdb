package dynamo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/repository"
)

const maxProfileMutationAttempts = 3

// ProfileRepository is the DynamoDB-backed repository.ProfileRepository.
// profileTableName holds the profile items; usernameTableName holds
// { username -> user_id } claim items that enforce username uniqueness (a
// GSI can't). Writes keep the two in sync via TransactWriteItems.
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
func (r *ProfileRepository) Get(ctx context.Context, ownerID string) (*repository.Profile, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &r.profileTableName,
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: ownerID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("getting profile for user %q: %w", ownerID, err)
	}

	if len(out.Item) == 0 {
		return nil, repository.ErrNotFound
	}

	var p repository.Profile
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return nil, fmt.Errorf("unmarshalling profile for user %q: %w", ownerID, err)
	}

	return &p, nil
}

// Create implements repository.ProfileRepository. The profile item and the
// { username -> user_id } claim are written in one TransactWriteItems so a
// username is never half-claimed; conflicts are classified by
// mapProfileCreateConflict.
func (r *ProfileRepository) Create(ctx context.Context, p repository.Profile) (*repository.Profile, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("creating profile: %w", repository.ErrNoUserID)
	}
	p.OwnerID = ownerID
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
// Discoverable / DiscordUsername fields, leaving each nil when the profile
// shouldn't be in that index.
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

// mapProfileCreateConflict maps a Create TransactWriteItems cancellation to
// a sentinel. Items are [profilePut, claimPut]; ErrAlreadyExists takes
// priority over ErrUsernameTaken when both conditions fail (you can't
// create at all, so the username is moot). Returns nil if err isn't a
// conditional-check cancellation.
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

// Update implements repository.ProfileRepository. It goes through
// mutateProfile so AvatarPath and Version carry forward from the stored
// item; only the body-settable fields come from p.
func (r *ProfileRepository) Update(ctx context.Context, p repository.Profile) (*repository.Profile, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("updating profile: %w", repository.ErrNoUserID)
	}

	updated, err := r.mutateProfile(ctx, ownerID, func(existing *repository.Profile) error {
		existing.Username = p.Username
		existing.Discoverable = p.Discoverable
		existing.DiscordUsername = p.DiscordUsername
		existing.Bio = p.Bio
		existing.Links = p.Links
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if errors.Is(err, repository.ErrUsernameTaken) {
		return nil, repository.ErrUsernameTaken
	}
	if err != nil {
		return nil, fmt.Errorf("updating profile for user %q: %w", ownerID, err)
	}

	return updated, nil
}

// errProfileAvatarAlreadyAbsent signals ClearAvatarPath's mutateProfile
// closure found no AvatarPath set - ClearAvatarPath treats this as success,
// not an error. Mirrors errSwitchImageAlreadyAbsent.
var errProfileAvatarAlreadyAbsent = errors.New("avatar already absent from profile")

// SetAvatarPath implements repository.ProfileRepository.
func (r *ProfileRepository) SetAvatarPath(ctx context.Context, key repository.ProfileImageKey) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("setting avatar path: %w", repository.ErrNoUserID)
	}

	_, err := r.mutateProfile(ctx, ownerID, func(p *repository.Profile) error {
		p.AvatarPath = &key
		return nil
	})
	if errors.Is(err, repository.ErrNotFound) {
		return repository.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("setting avatar path for user %q: %w", ownerID, err)
	}

	return nil
}

// ClearAvatarPath implements repository.ProfileRepository.
func (r *ProfileRepository) ClearAvatarPath(ctx context.Context) (*repository.ProfileImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return nil, fmt.Errorf("clearing avatar path: %w", repository.ErrNoUserID)
	}

	var cleared *repository.ProfileImageKey
	_, err := r.mutateProfile(ctx, ownerID, func(p *repository.Profile) error {
		if p.AvatarPath == nil {
			return errProfileAvatarAlreadyAbsent
		}
		cleared = p.AvatarPath
		p.AvatarPath = nil
		return nil
	})
	if errors.Is(err, errProfileAvatarAlreadyAbsent) {
		return nil, nil //nolint:nilnil // no avatar already set is a valid, expected result
	}
	if errors.Is(err, repository.ErrNotFound) {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("clearing avatar path for user %q: %w", ownerID, err)
	}

	return cleared, nil
}

// mutateProfile is a Version-based CAS retry loop like
// [(*SwitchRepository).mutateSwitch], except the rewrite is a
// TransactWriteItems: when mutate changes Username, the { username ->
// user_id } claim must move atomically with the profile item (delete old,
// put new under attribute_not_exists). A same-username mutation writes no
// claim items. Fields mutate doesn't touch (AvatarPath, ...) carry forward
// from the stored item; the directory-GSI keys are recomputed each attempt.
func (r *ProfileRepository) mutateProfile(
	ctx context.Context,
	ownerID string,
	mutate func(p *repository.Profile) error,
) (*repository.Profile, error) {
	for range maxProfileMutationAttempts {
		p, err := r.Get(ctx, ownerID)
		if err != nil {
			return nil, err
		}

		oldUsername := p.Username

		if err := mutate(p); err != nil {
			return nil, err
		}

		expectedVersion := p.Version
		p.Version++
		p.OwnerID = ownerID
		setProfileDirectoryKeys(p)

		profileItem, err := attributevalue.MarshalMap(*p)
		if err != nil {
			return nil, fmt.Errorf("marshalling profile for user %q: %w", ownerID, err)
		}

		// expectedVersion 0 also matches a pre-Version item with no version
		// attribute, hence the attribute_not_exists branch.
		// attribute_exists(user_id) is AND-ed in so this Put can update or
		// migrate a legacy item but never create one: without it a mutation
		// racing a Delete of a version:0 profile would resurrect it via the
		// attribute_not_exists(version) branch.
		versionCondition := expression.Name("version").Equal(expression.Value(expectedVersion))
		if expectedVersion == 0 {
			versionCondition = versionCondition.Or(expression.AttributeNotExists(expression.Name("version")))
		}
		profileCondition := expression.AttributeExists(expression.Name("user_id")).And(versionCondition)
		expr, err := expression.NewBuilder().WithCondition(profileCondition).Build()
		if err != nil {
			return nil, fmt.Errorf("building profile mutation condition for user %q: %w", ownerID, err)
		}

		items := []types.TransactWriteItem{
			{Put: &types.Put{
				TableName:                 &r.profileTableName,
				Item:                      profileItem,
				ConditionExpression:       expr.Condition(),
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
			}},
		}
		if p.Username != oldUsername {
			claimItem, err := attributevalue.MarshalMap(struct {
				Username string `dynamodbav:"username"`
				UserID   string `dynamodbav:"user_id"`
			}{Username: p.Username, UserID: ownerID})
			if err != nil {
				return nil, fmt.Errorf("marshalling username claim for user %q: %w", ownerID, err)
			}

			// Condition the old-claim delete on user_id = ownerID, mirroring
			// Delete(): between the Get above and this transaction firing a
			// concurrent rename or delete+recreate can move the oldUsername
			// claim to a different user, and an unconditioned delete would
			// wipe it. A ConditionalCheckFailed here re-reads and retries.
			claimDeleteExpr, err := expression.NewBuilder().
				WithCondition(expression.Name("user_id").Equal(expression.Value(ownerID))).
				Build()
			if err != nil {
				return nil, fmt.Errorf("building profile claim delete condition for user %q: %w", ownerID, err)
			}

			items = append(items,
				types.TransactWriteItem{Delete: &types.Delete{
					TableName: &r.usernameTableName,
					Key: map[string]types.AttributeValue{
						"username": &types.AttributeValueMemberS{Value: oldUsername},
					},
					ConditionExpression:       claimDeleteExpr.Condition(),
					ExpressionAttributeNames:  claimDeleteExpr.Names(),
					ExpressionAttributeValues: claimDeleteExpr.Values(),
				}},
				types.TransactWriteItem{Put: &types.Put{
					TableName:           &r.usernameTableName,
					Item:                claimItem,
					ConditionExpression: aws.String("attribute_not_exists(username)"),
				}},
			)
		}

		_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: items})
		if err == nil {
			return p, nil
		}

		if mapped := mapProfileUpdateConflict(err, p.Username != oldUsername); mapped != nil {
			if errors.Is(mapped, errProfileVersionConflict) {
				log.FromContext(ctx).Warn("profile CAS retry",
					log.ProfileID, ownerID, "attempted_version", expectedVersion)
				continue
			}
			return nil, mapped
		}
		return nil, fmt.Errorf("mutating profile for user %q: %w", ownerID, err)
	}

	return nil, fmt.Errorf("mutating profile for user %q: %w", ownerID, repository.ErrMutationConflict)
}

// errProfileVersionConflict signals mutateProfile to retry; never returned
// to a caller.
var errProfileVersionConflict = errors.New("profile version CAS conflict")

// mapProfileUpdateConflict classifies a mutateProfile TransactWriteItems
// cancellation. With a username change the items are [profilePut,
// claimDelete, claimPut]:
//   - reason 2 (claimPut) is the new username being taken by a different
//     user; a version retry wouldn't free it, so it wins over the version
//     CAS at reason 0.
//   - reason 1 (claimDelete, conditioned on user_id = caller) failing means
//     the old claim moved to someone else between the Get and the
//     transaction - re-read and retry (errProfileVersionConflict).
//
// Otherwise the only item is the profile Put and any failure - a lost
// version CAS, or the profile deleted out from under us failing
// attribute_exists(user_id) - is treated as a retry; the fresh Get then
// either succeeds or returns ErrNotFound. Returns nil if err isn't a
// conditional-check cancellation.
func mapProfileUpdateConflict(err error, usernameChanged bool) error {
	txErr, ok := errors.AsType[*types.TransactionCanceledException](err)
	if !ok {
		return nil
	}

	failed := func(i int) bool {
		return i < len(txErr.CancellationReasons) &&
			txErr.CancellationReasons[i].Code != nil &&
			*txErr.CancellationReasons[i].Code == "ConditionalCheckFailed"
	}

	if usernameChanged && failed(2) {
		return repository.ErrUsernameTaken
	}
	if failed(0) || (usernameChanged && failed(1)) {
		return errProfileVersionConflict
	}
	return nil
}

// Delete implements repository.ProfileRepository. A two-item
// TransactWriteItems removes the profile item and its { username ->
// user_id } claim together. A missing profile is a no-op success
// (idempotent).
//
// Delete can't address the claim without first reading the profile for its
// username, so a concurrent rename (or a full delete + recreate) between
// that read and the transaction could point the claim delete at a
// username that has since moved to - or been reclaimed by - a different
// item. Both deletes are therefore conditioned:
//   - profile item: attribute_exists(user_id), so a profile deleted out
//     from under us fails the transaction rather than the delete silently
//     "succeeding" against nothing;
//   - claim item: user_id = caller, so a claim that has since been moved
//     or reissued to someone else is left alone.
//
// A conditioned failure (ConditionalCheckFailed) or a TransactionConflict
// both re-read and retry: the fresh Get either returns ErrNotFound (the
// profile is genuinely gone now - idempotent success) or the current
// username to try again against.
func (r *ProfileRepository) Delete(ctx context.Context) error {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return fmt.Errorf("deleting profile: %w", repository.ErrNoUserID)
	}

	for range maxProfileMutationAttempts {
		p, err := r.Get(ctx, ownerID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}

		claimExpr, err := expression.NewBuilder().
			WithCondition(expression.Name("user_id").Equal(expression.Value(ownerID))).
			Build()
		if err != nil {
			return fmt.Errorf("building profile claim delete condition for user %q: %w", ownerID, err)
		}

		_, err = r.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
			TransactItems: []types.TransactWriteItem{
				{Delete: &types.Delete{
					TableName:           &r.profileTableName,
					Key:                 map[string]types.AttributeValue{"user_id": &types.AttributeValueMemberS{Value: ownerID}},
					ConditionExpression: aws.String("attribute_exists(user_id)"),
				}},
				{Delete: &types.Delete{
					TableName:                 &r.usernameTableName,
					Key:                       map[string]types.AttributeValue{"username": &types.AttributeValueMemberS{Value: p.Username}},
					ConditionExpression:       claimExpr.Condition(),
					ExpressionAttributeNames:  claimExpr.Names(),
					ExpressionAttributeValues: claimExpr.Values(),
				}},
			},
		})
		if err == nil {
			return nil
		}

		if profileDeleteShouldRetry(err) {
			log.FromContext(ctx).Warn("profile delete CAS retry", log.ProfileID, ownerID)
			continue
		}
		return fmt.Errorf("deleting profile for user %q: %w", ownerID, err)
	}

	return fmt.Errorf("deleting profile for user %q: %w", ownerID, repository.ErrMutationConflict)
}

// profileDeleteShouldRetry reports whether a failed Delete transaction is a
// transient conflict worth another attempt from a fresh read: one of the
// conditioned deletes failed (a concurrent rename / delete / recreate
// landed), or DynamoDB canceled with TransactionConflict. Any other
// cancellation reason (throttling, validation, ...) is not retried.
func profileDeleteShouldRetry(err error) bool {
	txErr, ok := errors.AsType[*types.TransactionCanceledException](err)
	if !ok {
		return false
	}

	for _, reason := range txErr.CancellationReasons {
		if reason.Code == nil {
			continue
		}
		if *reason.Code == "ConditionalCheckFailed" || *reason.Code == "TransactionConflict" {
			return true
		}
	}
	return false
}

const (
	discoverableUsernameIndex = "DiscoverableUsernameIndex"
	discoverableDiscordIndex  = "DiscoverableDiscordIndex"
	directoryPKValue          = "1"
)

// directoryIndexKeys is each GSI's LastEvaluatedKey attribute set, used to
// reject a cursor minted for the other filter.
var directoryIndexKeys = map[string]map[string]struct{}{
	discoverableUsernameIndex: {"discoverable_pk": {}, "username": {}, "user_id": {}},
	discoverableDiscordIndex:  {"discord_pk": {}, "discord_username_lc": {}, "user_id": {}},
}

// ListPublic implements repository.ProfileRepository. discordPrefix routes
// to the discord index (begins_with, lowercased), else the username index;
// the handler guarantees at most one prefix. A bad/mismatched cursor is
// repository.ErrInvalidCursor.
func (r *ProfileRepository) ListPublic(
	ctx context.Context,
	usernamePrefix, discordPrefix string,
	limit int,
	cursor string,
) ([]repository.Profile, string, error) {
	startKey, err := decodeCursor(cursor)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %w", repository.ErrInvalidCursor, err)
	}

	indexName := discoverableUsernameIndex
	keyCond := expression.Key("discoverable_pk").Equal(expression.Value(directoryPKValue))
	if usernamePrefix != "" {
		keyCond = keyCond.And(expression.KeyBeginsWith(expression.Key("username"), usernamePrefix))
	}
	if discordPrefix != "" {
		indexName = discoverableDiscordIndex
		keyCond = expression.Key("discord_pk").Equal(expression.Value(directoryPKValue)).
			And(expression.KeyBeginsWith(expression.Key("discord_username_lc"), strings.ToLower(discordPrefix)))
	}

	if !cursorMatchesIndex(startKey, indexName) {
		return nil, "", fmt.Errorf("%w: minted for a different filter", repository.ErrInvalidCursor)
	}

	expr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
	if err != nil {
		return nil, "", fmt.Errorf("building profile directory expression: %w", err)
	}

	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 &r.profileTableName,
		IndexName:                 &indexName,
		KeyConditionExpression:    expr.KeyCondition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		ExclusiveStartKey:         startKey,
		Limit:                     queryLimit(limit),
	})
	if err != nil {
		return nil, "", fmt.Errorf("querying discoverable profiles: %w", err)
	}

	profiles := []repository.Profile{}
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &profiles); err != nil {
		return nil, "", fmt.Errorf("unmarshalling discoverable profiles: %w", err)
	}

	nextCursor, err := encodeCursor(out.LastEvaluatedKey)
	if err != nil {
		return nil, "", fmt.Errorf("encoding next cursor: %w", err)
	}

	return profiles, nextCursor, nil
}

// cursorMatchesIndex reports whether startKey's attributes are exactly
// directoryIndexKeys[indexName]. An empty key always matches.
func cursorMatchesIndex(startKey map[string]types.AttributeValue, indexName string) bool {
	if len(startKey) == 0 {
		return true
	}

	want := directoryIndexKeys[indexName]
	if len(startKey) != len(want) {
		return false
	}
	for k := range startKey {
		if _, ok := want[k]; !ok {
			return false
		}
	}
	return true
}

// ResolveUsername implements repository.ProfileRepository, reading the
// { username -> user_id } claim item. ErrNotFound means the username is
// unclaimed.
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
