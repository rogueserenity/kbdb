package dynamo

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/dynamo/mocks"
)

type SwitchRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *SwitchRepository
}

func TestSwitchRepositorySuite(t *testing.T) {
	suite.Run(t, new(SwitchRepositorySuite))
}

func (s *SwitchRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &SwitchRepository{client: s.mockClient, tableName: "switch-table"}
}

func (s *SwitchRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == 20
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id": &types.AttributeValueMemberS{Value: "alice"},
					"id":      &types.AttributeValueMemberS{Value: "sw1"},
					"brand":   &types.AttributeValueMemberS{Value: "Gateron"},
				},
			},
		}, nil)

	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(switches, 1)
	s.Equal("sw1", switches[0].ID)
	s.Equal("Gateron", switches[0].Brand)
}

func (s *SwitchRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	// No EXPECT() on s.mockClient.Query - an empty visibilities slice must
	// short-circuit before building a Query, since expression.In(...)
	// requires at least one value and would otherwise panic.
	switches, next, err := s.repo.List(s.T().Context(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(switches)
	s.Empty(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	switches, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotNil(switches)
	s.Empty(switches)
}

func (s *SwitchRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			},
		}, nil)

	_, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *SwitchRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
	// Round-trip: encode a key via a first call, then confirm the second
	// call's ExclusiveStartKey matches what was encoded.
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "sw1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestList_InvalidCursor_ReturnsError() {
	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().Error(err)
	s.Nil(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	switches, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().Error(err)
	s.Nil(switches)
	s.Empty(next)
}

func (s *SwitchRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "sw1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "sw1"},
				"brand":   &types.AttributeValueMemberS{Value: "Gateron"},
			},
		}, nil)

	sw, err := s.repo.Get(s.T().Context(), "alice", "sw1")

	s.Require().NoError(err)
	s.Equal("sw1", sw.ID)
	s.Equal("Gateron", sw.Brand)
}

func (s *SwitchRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	sw, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	sw, err := s.repo.Get(s.T().Context(), "alice", "sw1")

	s.Require().Error(err)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.Equal(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron"}, sw)
}

func (s *SwitchRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Create(ctx, repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see repository.ErrNoUserID.
	sw, err := s.repo.Create(s.T().Context(), repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			return *in.ConditionExpression == "attribute_exists(id)"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1", Brand: "Gateron"})

	s.Require().NoError(err)
	s.Equal(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron"}, sw)
}

func (s *SwitchRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestUpdate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	sw, err := s.repo.Update(ctx, repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see repository.ErrNoUserID.
	sw, err := s.repo.Update(s.T().Context(), repository.Switch{ID: "sw1"})

	s.Require().Error(err)
	s.Nil(sw)
}

func (s *SwitchRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "sw1")

	s.Require().NoError(err)
}

func (s *SwitchRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on DeleteItem - see repository.ErrNoUserID.
	err := s.repo.Delete(s.T().Context(), "sw1")

	s.Require().Error(err)
}

func (s *SwitchRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "sw1")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
}
