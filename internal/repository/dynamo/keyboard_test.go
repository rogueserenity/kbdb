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

type KeyboardRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *KeyboardRepository
}

func TestKeyboardRepositorySuite(t *testing.T) {
	suite.Run(t, new(KeyboardRepositorySuite))
}

func (s *KeyboardRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &KeyboardRepository{client: s.mockClient, tableName: "keyboard-table"}
}

func (s *KeyboardRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == 20
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id": &types.AttributeValueMemberS{Value: "alice"},
					"id":      &types.AttributeValueMemberS{Value: "kb1"},
					"brand":   &types.AttributeValueMemberS{Value: "Keychron"},
				},
			},
		}, nil)

	keyboards, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(keyboards, 1)
	s.Equal("kb1", keyboards[0].ID)
	s.Equal("Keychron", keyboards[0].Brand)
}

func (s *KeyboardRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	// No EXPECT() on s.mockClient.Query - an empty visibilities slice must
	// short-circuit before building a Query, since expression.In(...)
	// requires at least one value and would otherwise panic.
	keyboards, next, err := s.repo.List(s.T().Context(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(keyboards)
	s.Empty(keyboards)
	s.Empty(next)
}

func (s *KeyboardRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	keyboards, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotNil(keyboards)
	s.Empty(keyboards)
}

func (s *KeyboardRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "kb1"},
			},
		}, nil)

	_, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *KeyboardRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
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
				"id":      &types.AttributeValueMemberS{Value: "kb1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "kb1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *KeyboardRepositorySuite) TestList_InvalidCursor_ReturnsError() {
	keyboards, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().Error(err)
	s.Nil(keyboards)
	s.Empty(next)
}

func (s *KeyboardRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	keyboards, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().Error(err)
	s.Nil(keyboards)
	s.Empty(next)
}

func (s *KeyboardRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "kb1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "kb1"},
				"brand":   &types.AttributeValueMemberS{Value: "Keychron"},
			},
		}, nil)

	kb, err := s.repo.Get(s.T().Context(), "alice", "kb1")

	s.Require().NoError(err)
	s.Equal("kb1", kb.ID)
	s.Equal("Keychron", kb.Brand)
}

func (s *KeyboardRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	kb, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	kb, err := s.repo.Get(s.T().Context(), "alice", "kb1")

	s.Require().Error(err)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Create(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Equal(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron"}, kb)
}

func (s *KeyboardRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Create(ctx, repository.Keyboard{ID: "kb1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Create(ctx, repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see errNoUserID (client.go).
	kb, err := s.repo.Create(s.T().Context(), repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			return *in.ConditionExpression == "attribute_exists(id)"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Equal(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron"}, kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see errNoUserID (client.go).
	kb, err := s.repo.Update(s.T().Context(), repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "kb1")

	s.Require().NoError(err)
}

func (s *KeyboardRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on DeleteItem - see errNoUserID (client.go).
	err := s.repo.Delete(s.T().Context(), "kb1")

	s.Require().Error(err)
}

func (s *KeyboardRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "kb1")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
}
