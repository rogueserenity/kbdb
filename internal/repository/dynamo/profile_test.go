package dynamo

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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
