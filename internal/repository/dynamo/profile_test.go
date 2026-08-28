package dynamo

import (
	"context"
	"errors"
	"maps"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/dynamo/mocks"
)

type ProfileRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *ProfileRepository
}

func TestProfileRepositorySuite(t *testing.T) {
	suite.Run(t, new(ProfileRepositorySuite))
}

func (s *ProfileRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &ProfileRepository{
		client:            s.mockClient,
		profileTableName:  "profile-table",
		usernameTableName: "profile-username-table",
	}
}

func (s *ProfileRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			return *in.TableName == "profile-table" && ok && userID.Value == "user-alice" && len(in.Key) == 1
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id":      &types.AttributeValueMemberS{Value: "user-alice"},
				"username":     &types.AttributeValueMemberS{Value: "alice"},
				"discoverable": &types.AttributeValueMemberBOOL{Value: true},
				"bio":          &types.AttributeValueMemberS{Value: "keebs enjoyer"},
			},
		}, nil)

	p, err := s.repo.Get(s.T().Context(), "user-alice")

	s.Require().NoError(err)
	s.Equal("user-alice", p.OwnerID)
	s.Equal("alice", p.Username)
	s.True(p.Discoverable)
	s.Require().NotNil(p.Bio)
	s.Equal("keebs enjoyer", *p.Bio)
}

func (s *ProfileRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	_, err := s.repo.Get(s.T().Context(), "user-nobody")

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *ProfileRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("boom"))

	_, err := s.repo.Get(s.T().Context(), "user-alice")

	s.Require().Error(err)
	s.NotErrorIs(err, repository.ErrNotFound)
}

// unmarshalProfileItem decodes the profile Put's item (TransactItems[0])
// back into a repository.Profile for assertions on the derived attributes.
func (s *ProfileRepositorySuite) unmarshalProfileItem(in *dynamodb.TransactWriteItemsInput) repository.Profile {
	s.T().Helper()
	var p repository.Profile
	s.Require().NoError(attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &p))
	return p
}

func (s *ProfileRepositorySuite) TestCreate_NoUserID_ReturnsErrNoUserID() {
	_, err := s.repo.Create(s.T().Context(), repository.Profile{Username: "alice"})

	s.Require().ErrorIs(err, repository.ErrNoUserID)
}

func (s *ProfileRepositorySuite) TestCreate_WritesProfileAndClaimAtomically() {
	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return len(in.TransactItems) == 2 &&
				in.TransactItems[0].Put != nil && in.TransactItems[1].Put != nil
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	got, err := s.repo.Create(ctx, repository.Profile{Username: "alice"})

	s.Require().NoError(err)
	s.Equal("user-alice", got.OwnerID)
	s.Equal(0, got.Version)

	profilePut := captured.TransactItems[0].Put
	s.Equal("profile-table", *profilePut.TableName)
	s.Equal("attribute_not_exists(user_id)", *profilePut.ConditionExpression)

	claimPut := captured.TransactItems[1].Put
	s.Equal("profile-username-table", *claimPut.TableName)
	s.Equal("attribute_not_exists(username)", *claimPut.ConditionExpression)
	s.Equal("alice", claimPut.Item["username"].(*types.AttributeValueMemberS).Value)
	s.Equal("user-alice", claimPut.Item["user_id"].(*types.AttributeValueMemberS).Value)
}

func (s *ProfileRepositorySuite) TestCreate_NonDiscoverable_OmitsAllGSIKeys() {
	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	discord := "Alice_KB"
	_, err := s.repo.Create(ctx, repository.Profile{
		Username: "alice", Discoverable: false, DiscordUsername: &discord,
	})
	s.Require().NoError(err)

	item := captured.TransactItems[0].Put.Item
	s.NotContains(item, "discoverable_pk")
	s.NotContains(item, "discord_pk")
	s.NotContains(item, "discord_username_lc")
}

func (s *ProfileRepositorySuite) TestCreate_DiscoverableNoDiscord_SetsOnlyDiscoverablePK() {
	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	_, err := s.repo.Create(ctx, repository.Profile{Username: "alice", Discoverable: true})
	s.Require().NoError(err)

	p := s.unmarshalProfileItem(captured)
	s.Require().NotNil(p.DiscoverablePK)
	s.Equal("1", *p.DiscoverablePK)
	s.Nil(p.DiscordPK)
	s.Nil(p.DiscordUsernameLC)
}

func (s *ProfileRepositorySuite) TestCreate_DiscoverableWithDiscord_SetsAllGSIKeysLowercased() {
	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	discord := "Alice_KB"
	_, err := s.repo.Create(ctx, repository.Profile{
		Username: "alice", Discoverable: true, DiscordUsername: &discord,
	})
	s.Require().NoError(err)

	p := s.unmarshalProfileItem(captured)
	s.Require().NotNil(p.DiscoverablePK)
	s.Equal("1", *p.DiscoverablePK)
	s.Require().NotNil(p.DiscordPK)
	s.Equal("1", *p.DiscordPK)
	s.Require().NotNil(p.DiscordUsernameLC)
	s.Equal("alice_kb", *p.DiscordUsernameLC)
}

func (s *ProfileRepositorySuite) TestCreate_ProfilePutConditionFails_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("ConditionalCheckFailed")},
				{Code: aws.String("None")},
			},
		})

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	_, err := s.repo.Create(ctx, repository.Profile{Username: "alice"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.NotErrorIs(err, repository.ErrUsernameTaken)
}

func (s *ProfileRepositorySuite) TestCreate_ClaimPutConditionFails_ReturnsErrUsernameTaken() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("None")},
				{Code: aws.String("ConditionalCheckFailed")},
			},
		})

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	_, err := s.repo.Create(ctx, repository.Profile{Username: "alice"})

	s.Require().ErrorIs(err, repository.ErrUsernameTaken)
	s.NotErrorIs(err, repository.ErrAlreadyExists)
}

func (s *ProfileRepositorySuite) TestCreate_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "user-alice")
	_, err := s.repo.Create(ctx, repository.Profile{Username: "alice"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Require().NotErrorIs(err, repository.ErrUsernameTaken)
}

// storedProfileItem is a GetItemOutput for a profile at the given version,
// with the fields mutateProfile carries forward populated so a test can
// assert they survive the rewrite.
func storedProfileItem(version int, extra map[string]types.AttributeValue) *dynamodb.GetItemOutput {
	item := map[string]types.AttributeValue{
		"user_id":      &types.AttributeValueMemberS{Value: "user-alice"},
		"username":     &types.AttributeValueMemberS{Value: "alice"},
		"discoverable": &types.AttributeValueMemberBOOL{Value: true},
		"version":      &types.AttributeValueMemberN{Value: strconv.Itoa(version)},
	}
	maps.Copy(item, extra)
	return &dynamodb.GetItemOutput{Item: item}
}

func (s *ProfileRepositorySuite) updateCtx() context.Context {
	return kbdbctx.WithUserID(s.T().Context(), "user-alice")
}

func (s *ProfileRepositorySuite) TestUpdate_NoUserID_ReturnsErrNoUserID() {
	_, err := s.repo.Update(s.T().Context(), repository.Profile{Username: "alice"})

	s.Require().ErrorIs(err, repository.ErrNoUserID)
}

func (s *ProfileRepositorySuite) TestUpdate_SameUsername_WritesOnlyProfileItem_NoClaimWrites() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	newBio := "updated"
	got, err := s.repo.Update(s.updateCtx(), repository.Profile{
		Username: "alice", Discoverable: true, Bio: &newBio,
	})

	s.Require().NoError(err)
	s.Len(captured.TransactItems, 1)
	s.NotNil(captured.TransactItems[0].Put)
	s.Equal("profile-table", *captured.TransactItems[0].Put.TableName)
	s.Equal(1, got.Version)
	s.Require().NotNil(got.Bio)
	s.Equal("updated", *got.Bio)
}

func (s *ProfileRepositorySuite) TestUpdate_OmittingBioAndLinks_ClearsThem() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, map[string]types.AttributeValue{
			"bio":   &types.AttributeValueMemberS{Value: "old bio"},
			"links": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
		}), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	_, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice", Discoverable: true})
	s.Require().NoError(err)

	item := captured.TransactItems[0].Put.Item
	s.NotContains(item, "bio")
	s.NotContains(item, "links")
}

func (s *ProfileRepositorySuite) TestUpdate_BodyDoesNotMentionAvatar_CarriesAvatarPathForward() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, map[string]types.AttributeValue{
			"avatar_path": &types.AttributeValueMemberS{Value: "profiles/user-alice/avatar"},
		}), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	got, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice", Discoverable: true})
	s.Require().NoError(err)

	item := captured.TransactItems[0].Put.Item
	s.Equal("profiles/user-alice/avatar",
		item["avatar_path"].(*types.AttributeValueMemberS).Value)
	s.Require().NotNil(got.AvatarPath)
	s.Equal(repository.ProfileImageKey("profiles/user-alice/avatar"), *got.AvatarPath)
}

func (s *ProfileRepositorySuite) TestUpdate_FlipDiscoverableFalse_RemovesAllGSIKeys() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, map[string]types.AttributeValue{
			"discoverable_pk":     &types.AttributeValueMemberS{Value: "1"},
			"discord_pk":          &types.AttributeValueMemberS{Value: "1"},
			"discord_username_lc": &types.AttributeValueMemberS{Value: "alice_kb"},
			"discord_username":    &types.AttributeValueMemberS{Value: "Alice_KB"},
		}), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	discord := "Alice_KB"
	_, err := s.repo.Update(s.updateCtx(), repository.Profile{
		Username: "alice", Discoverable: false, DiscordUsername: &discord,
	})
	s.Require().NoError(err)

	item := captured.TransactItems[0].Put.Item
	s.NotContains(item, "discoverable_pk")
	s.NotContains(item, "discord_pk")
	s.NotContains(item, "discord_username_lc")
}

func (s *ProfileRepositorySuite) TestUpdate_ChangedUsername_MovesClaimAtomically() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	_, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice2", Discoverable: true})
	s.Require().NoError(err)

	s.Require().Len(captured.TransactItems, 3)
	s.NotNil(captured.TransactItems[0].Put)
	s.Require().NotNil(captured.TransactItems[1].Delete)
	s.Equal("profile-username-table", *captured.TransactItems[1].Delete.TableName)
	s.Equal("alice",
		captured.TransactItems[1].Delete.Key["username"].(*types.AttributeValueMemberS).Value)
	// The old-claim delete is conditioned on user_id = caller so a
	// concurrent rename / delete+recreate that moved the claim can't be
	// clobbered (mirrors Delete()).
	claimDel := captured.TransactItems[1].Delete
	s.Require().NotNil(claimDel.ConditionExpression)
	s.Require().Len(claimDel.ExpressionAttributeNames, 1)
	s.Require().Len(claimDel.ExpressionAttributeValues, 1)
	for _, name := range claimDel.ExpressionAttributeNames {
		s.Equal("user_id", name)
	}
	for _, val := range claimDel.ExpressionAttributeValues {
		s.Equal("user-alice", val.(*types.AttributeValueMemberS).Value)
	}
	s.Require().NotNil(captured.TransactItems[2].Put)
	s.Equal("profile-username-table", *captured.TransactItems[2].Put.TableName)
	s.Equal("attribute_not_exists(username)", *captured.TransactItems[2].Put.ConditionExpression)
	s.Equal("alice2",
		captured.TransactItems[2].Put.Item["username"].(*types.AttributeValueMemberS).Value)
}

func (s *ProfileRepositorySuite) TestUpdate_NoProfile_ReturnsErrNotFound() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	_, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *ProfileRepositorySuite) TestUpdate_ChangedUsernameClaimFails_ReturnsErrUsernameTaken() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("None")},
				{Code: aws.String("None")},
				{Code: aws.String("ConditionalCheckFailed")},
			},
		})

	_, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "taken", Discoverable: true})

	s.Require().ErrorIs(err, repository.ErrUsernameTaken)
}

// A ConditionalCheckFailed on the old-claim delete (reason 1) means the
// "alice" claim moved to another user between the Get and the transaction:
// re-read and retry, don't surface it as an error.
func (s *ProfileRepositorySuite) TestUpdate_ChangedUsernameOldClaimMoved_RetriesThenSucceeds() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil).Once()
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(1, nil), nil).Once()

	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("None")},
				{Code: aws.String("ConditionalCheckFailed")},
				{Code: aws.String("None")},
			},
		}).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	got, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice2", Discoverable: true})

	s.Require().NoError(err)
	s.Equal(2, got.Version)
}

func (s *ProfileRepositorySuite) TestUpdate_VersionCASConflict_RetriesThenSucceeds() {
	gomock := s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything)
	gomock.Return(storedProfileItem(0, nil), nil).Once()
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(1, nil), nil).Once()

	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("ConditionalCheckFailed")},
			},
		}).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	newBio := "second try wins"
	got, err := s.repo.Update(s.updateCtx(), repository.Profile{
		Username: "alice", Discoverable: true, Bio: &newBio,
	})

	s.Require().NoError(err)
	s.Equal(2, got.Version)
}

func (s *ProfileRepositorySuite) TestUpdate_VersionCASConflictExhaustsRetries_ReturnsErrMutationConflict() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("ConditionalCheckFailed")},
			},
		})

	_, err := s.repo.Update(s.updateCtx(), repository.Profile{Username: "alice", Discoverable: true})

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
}

func (s *ProfileRepositorySuite) TestSetAvatarPath_NoUserID_ReturnsErrNoUserID() {
	err := s.repo.SetAvatarPath(s.T().Context(), "profiles/user-alice/avatar")

	s.Require().ErrorIs(err, repository.ErrNoUserID)
}

func (s *ProfileRepositorySuite) TestSetAvatarPath_SetsKeyViaWholeItemPut() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, map[string]types.AttributeValue{
			"discoverable_pk":     &types.AttributeValueMemberS{Value: "1"},
			"discord_pk":          &types.AttributeValueMemberS{Value: "1"},
			"discord_username_lc": &types.AttributeValueMemberS{Value: "alice_kb"},
			"discord_username":    &types.AttributeValueMemberS{Value: "Alice_KB"},
		}), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	err := s.repo.SetAvatarPath(s.updateCtx(), "profiles/user-alice/avatar")
	s.Require().NoError(err)

	// Only the profile item is written - no username change, so no claim moves.
	s.Require().Len(captured.TransactItems, 1)
	item := captured.TransactItems[0].Put.Item
	s.Equal("profiles/user-alice/avatar", item["avatar_path"].(*types.AttributeValueMemberS).Value)

	// The whole-item Put must not drop the sparse-GSI attributes or username
	// when it only touches avatar_path.
	s.Equal("1", item["discoverable_pk"].(*types.AttributeValueMemberS).Value)
	s.Equal("1", item["discord_pk"].(*types.AttributeValueMemberS).Value)
	s.Equal("alice_kb", item["discord_username_lc"].(*types.AttributeValueMemberS).Value)
	s.Equal("alice", item["username"].(*types.AttributeValueMemberS).Value)
}

func (s *ProfileRepositorySuite) TestSetAvatarPath_NoProfile_ReturnsErrNotFound() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	err := s.repo.SetAvatarPath(s.updateCtx(), "profiles/user-alice/avatar")

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *ProfileRepositorySuite) TestClearAvatarPath_NoUserID_ReturnsErrNoUserID() {
	_, err := s.repo.ClearAvatarPath(s.T().Context())

	s.Require().ErrorIs(err, repository.ErrNoUserID)
}

func (s *ProfileRepositorySuite) TestClearAvatarPath_ClearsKeyAndReturnsIt_KeepingSparseGSIAttrs() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, map[string]types.AttributeValue{
			"avatar_path":         &types.AttributeValueMemberS{Value: "profiles/user-alice/avatar"},
			"discoverable_pk":     &types.AttributeValueMemberS{Value: "1"},
			"discord_pk":          &types.AttributeValueMemberS{Value: "1"},
			"discord_username_lc": &types.AttributeValueMemberS{Value: "alice_kb"},
			"discord_username":    &types.AttributeValueMemberS{Value: "Alice_KB"},
		}), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	cleared, err := s.repo.ClearAvatarPath(s.updateCtx())
	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.ProfileImageKey("profiles/user-alice/avatar"), *cleared)

	item := captured.TransactItems[0].Put.Item
	s.NotContains(item, "avatar_path")
	s.Equal("1", item["discoverable_pk"].(*types.AttributeValueMemberS).Value)
	s.Equal("1", item["discord_pk"].(*types.AttributeValueMemberS).Value)
	s.Equal("alice_kb", item["discord_username_lc"].(*types.AttributeValueMemberS).Value)
	s.Equal("alice", item["username"].(*types.AttributeValueMemberS).Value)
}

func (s *ProfileRepositorySuite) TestClearAvatarPath_NoAvatarSet_ReturnsNilWithoutWriting() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)
	// No EXPECT() on TransactWriteItems - an absent avatar is a no-op, not a write.

	cleared, err := s.repo.ClearAvatarPath(s.updateCtx())

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *ProfileRepositorySuite) TestClearAvatarPath_NoProfile_ReturnsErrNotFound() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	cleared, err := s.repo.ClearAvatarPath(s.updateCtx())

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *ProfileRepositorySuite) TestDelete_NoUserID_ReturnsErrNoUserID() {
	err := s.repo.Delete(s.T().Context())

	s.Require().ErrorIs(err, repository.ErrNoUserID)
}

func (s *ProfileRepositorySuite) TestDelete_RemovesProfileAndClaimAtomically_BothConditioned() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	err := s.repo.Delete(s.updateCtx())

	s.Require().NoError(err)
	s.Require().Len(captured.TransactItems, 2)

	profileDel := captured.TransactItems[0].Delete
	s.Require().NotNil(profileDel)
	s.Equal("profile-table", *profileDel.TableName)
	s.Equal("user-alice", profileDel.Key["user_id"].(*types.AttributeValueMemberS).Value)
	// Guards against deleting nothing when the profile was removed under us.
	s.Require().NotNil(profileDel.ConditionExpression)
	s.Contains(*profileDel.ConditionExpression, "attribute_exists")

	claimDel := captured.TransactItems[1].Delete
	s.Require().NotNil(claimDel)
	s.Equal("profile-username-table", *claimDel.TableName)
	s.Equal("alice", claimDel.Key["username"].(*types.AttributeValueMemberS).Value)
	// Guards against deleting a claim that has since been reissued to
	// someone else (a rename or delete+recreate race).
	s.Require().NotNil(claimDel.ConditionExpression)
	ownerVal := ""
	for _, v := range claimDel.ExpressionAttributeValues {
		if sv, ok := v.(*types.AttributeValueMemberS); ok {
			ownerVal = sv.Value
		}
	}
	s.Equal("user-alice", ownerVal)
}

func (s *ProfileRepositorySuite) TestDelete_ConcurrentRename_RetriesWithFreshUsername() {
	// Attempt 1: profile still reads as "alice"; the claim-delete condition
	// fails because a rename already moved the "alice" claim.
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("None")},
				{Code: aws.String("ConditionalCheckFailed")},
			},
		}).Once()

	// Attempt 2: fresh Get returns the renamed profile; delete targets the
	// new claim and succeeds.
	renamed := storedProfileItem(1, map[string]types.AttributeValue{
		"username": &types.AttributeValueMemberS{Value: "bob"},
	})
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).Return(renamed, nil).Once()

	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	err := s.repo.Delete(s.updateCtx())

	s.Require().NoError(err)
	s.Equal("bob", captured.TransactItems[1].Delete.Key["username"].(*types.AttributeValueMemberS).Value)
}

func (s *ProfileRepositorySuite) TestDelete_TransactionConflict_Retries() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("TransactionConflict")},
				{Code: aws.String("None")},
			},
		}).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	err := s.repo.Delete(s.updateCtx())

	s.Require().NoError(err)
}

func (s *ProfileRepositorySuite) TestDelete_ConflictExhaustsRetries_ReturnsErrMutationConflict() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{
				{Code: aws.String("ConditionalCheckFailed")},
				{Code: aws.String("None")},
			},
		})

	err := s.repo.Delete(s.updateCtx())

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
}

func (s *ProfileRepositorySuite) TestDelete_NoProfile_IsNoOpSuccess() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	err := s.repo.Delete(s.updateCtx())

	s.Require().NoError(err)
	s.mockClient.AssertNotCalled(s.T(), "TransactWriteItems", mock.Anything, mock.Anything)
}

func (s *ProfileRepositorySuite) TestDelete_GetError_Propagates() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	err := s.repo.Delete(s.updateCtx())

	s.Require().Error(err)
	s.mockClient.AssertNotCalled(s.T(), "TransactWriteItems", mock.Anything, mock.Anything)
}

func (s *ProfileRepositorySuite) TestDelete_NonRetryableError_PropagatesWithoutRetry() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(storedProfileItem(0, nil), nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled")).Once()

	err := s.repo.Delete(s.updateCtx())

	s.Require().Error(err)
	s.NotErrorIs(err, repository.ErrMutationConflict)
}

func (s *ProfileRepositorySuite) TestResolveUsername_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			username, ok := in.Key["username"].(*types.AttributeValueMemberS)
			return *in.TableName == "profile-username-table" && ok && username.Value == "alice"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"username": &types.AttributeValueMemberS{Value: "alice"},
				"user_id":  &types.AttributeValueMemberS{Value: "user-alice"},
			},
		}, nil)

	got, err := s.repo.ResolveUsername(s.T().Context(), "alice")

	s.Require().NoError(err)
	s.Equal("user-alice", got)
}

func (s *ProfileRepositorySuite) TestResolveUsername_NoClaim_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	_, err := s.repo.ResolveUsername(s.T().Context(), "ghost")

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *ProfileRepositorySuite) TestResolveUsername_ClaimMissingUserID_IsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"username": &types.AttributeValueMemberS{Value: "alice"},
			},
		}, nil)

	_, err := s.repo.ResolveUsername(s.T().Context(), "alice")

	s.Require().Error(err)
	s.NotErrorIs(err, repository.ErrNotFound)
}

func discoverableRow(userID, username string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"user_id":         &types.AttributeValueMemberS{Value: userID},
		"username":        &types.AttributeValueMemberS{Value: username},
		"discoverable":    &types.AttributeValueMemberBOOL{Value: true},
		"discoverable_pk": &types.AttributeValueMemberS{Value: "1"},
	}
}

func (s *ProfileRepositorySuite) TestListPublic_QueriesUsernameIndex_NoPrefix() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.TableName == "profile-table" &&
				in.IndexName != nil && *in.IndexName == "DiscoverableUsernameIndex" &&
				*in.Limit == 20 && len(in.ExclusiveStartKey) == 0 &&
				!containsAttrName(in.ExpressionAttributeNames, "username")
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				discoverableRow("user-alice", "alice"),
				discoverableRow("user-bob", "bob"),
			},
		}, nil)

	profiles, next, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(profiles, 2)
	s.Equal("alice", profiles[0].Username)
	s.Equal("user-alice", profiles[0].OwnerID)
	s.True(profiles[0].Discoverable)
}

func containsAttrName(names map[string]string, attr string) bool {
	for _, v := range names {
		if v == attr {
			return true
		}
	}
	return false
}

func (s *ProfileRepositorySuite) TestListPublic_UsernamePrefix_AddsBeginsWith() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.IndexName == "DiscoverableUsernameIndex" &&
				containsAttrName(in.ExpressionAttributeNames, "username") &&
				exprValuesContainS(in.ExpressionAttributeValues, "al")
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{discoverableRow("user-alice", "alice")},
		}, nil)

	profiles, _, err := s.repo.ListPublic(s.T().Context(), "al", "", 20, "")

	s.Require().NoError(err)
	s.Require().Len(profiles, 1)
	s.Equal("alice", profiles[0].Username)
}

func (s *ProfileRepositorySuite) TestListPublic_DiscordPrefix_UsesDiscordIndex_Lowercased() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return *in.IndexName == "DiscoverableDiscordIndex" &&
				containsAttrName(in.ExpressionAttributeNames, "discord_pk") &&
				containsAttrName(in.ExpressionAttributeNames, "discord_username_lc") &&
				exprValuesContainS(in.ExpressionAttributeValues, "cool") // lowercased
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{discoverableRow("user-alice", "alice")},
		}, nil)

	_, _, err := s.repo.ListPublic(s.T().Context(), "", "CooL", 20, "")

	s.Require().NoError(err)
}

func exprValuesContainS(values map[string]types.AttributeValue, want string) bool {
	for _, v := range values {
		if sv, ok := v.(*types.AttributeValueMemberS); ok && sv.Value == want {
			return true
		}
	}
	return false
}

func (s *ProfileRepositorySuite) TestListPublic_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	profiles, _, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "")

	s.Require().NoError(err)
	s.NotNil(profiles)
	s.Empty(profiles)
}

func (s *ProfileRepositorySuite) TestListPublic_Cursor_RoundTripsThreeKeyGSIKey() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id":         &types.AttributeValueMemberS{Value: "user-bob"},
				"discoverable_pk": &types.AttributeValueMemberS{Value: "1"},
				"username":        &types.AttributeValueMemberS{Value: "bob"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "")
	s.Require().NoError(err)
	s.Require().NotEmpty(cursor)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			k := in.ExclusiveStartKey
			uid, ok1 := k["user_id"].(*types.AttributeValueMemberS)
			pk, ok2 := k["discoverable_pk"].(*types.AttributeValueMemberS)
			un, ok3 := k["username"].(*types.AttributeValueMemberS)
			return len(k) == 3 && ok1 && ok2 && ok3 &&
				uid.Value == "user-bob" && pk.Value == "1" && un.Value == "bob"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.ListPublic(s.T().Context(), "", "", 20, cursor)
	s.Require().NoError(err)
}

func (s *ProfileRepositorySuite) TestListPublic_InvalidCursor_ReturnsErrInvalidCursor() {
	profiles, next, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "not-valid-base64!!")

	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
	s.Nil(profiles)
	s.Empty(next)
}

func (s *ProfileRepositorySuite) TestListPublic_CursorFromOtherFilter_ReturnsErrInvalidCursor() {
	// Page 1: no filter -> a username-index cursor.
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id":         &types.AttributeValueMemberS{Value: "user-bob"},
				"discoverable_pk": &types.AttributeValueMemberS{Value: "1"},
				"username":        &types.AttributeValueMemberS{Value: "bob"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "")
	s.Require().NoError(err)

	// Reusing it with a discord_username filter must be rejected, not passed
	// to Query as a mismatched ExclusiveStartKey.
	_, _, err = s.repo.ListPublic(s.T().Context(), "", "cool", 20, cursor)
	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
}

func (s *ProfileRepositorySuite) TestListPublic_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	_, _, err := s.repo.ListPublic(s.T().Context(), "", "", 20, "")

	s.Require().Error(err)
}
