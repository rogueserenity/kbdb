package dynamo

import (
	"errors"
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
	s.Equal("user-alice", p.StytchUserID)
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
	s.Equal("user-alice", got.StytchUserID)
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
