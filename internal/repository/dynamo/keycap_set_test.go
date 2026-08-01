package dynamo

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/dynamo/mocks"
)

type KeycapSetRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *KeycapSetRepository
}

func TestKeycapSetRepositorySuite(t *testing.T) {
	suite.Run(t, new(KeycapSetRepositorySuite))
}

func (s *KeycapSetRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &KeycapSetRepository{client: s.mockClient, tableName: "keycap-set-table"}
}

func (s *KeycapSetRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == 20
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id": &types.AttributeValueMemberS{Value: "alice"},
					"id":      &types.AttributeValueMemberS{Value: "ks1"},
					"brand":   &types.AttributeValueMemberS{Value: "GMK"},
				},
			},
		}, nil)

	sets, next, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(sets, 1)
	s.Equal("ks1", sets[0].ID)
	s.Equal("GMK", sets[0].Brand)
}

func (s *KeycapSetRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	// No EXPECT() on s.mockClient.Query - an empty visibilities slice must
	// short-circuit before building a Query, since expression.In(...)
	// requires at least one value and would otherwise panic.
	sets, next, err := s.repo.List(context.Background(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(sets)
	s.Empty(sets)
	s.Empty(next)
}

func (s *KeycapSetRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	sets, _, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotNil(sets)
	s.Empty(sets)
}

func (s *KeycapSetRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "ks1"},
			},
		}, nil)

	_, next, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *KeycapSetRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
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
				"id":      &types.AttributeValueMemberS{Value: "ks1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "ks1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *KeycapSetRepositorySuite) TestList_InvalidCursor_ReturnsError() {
	sets, next, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().Error(err)
	s.Nil(sets)
	s.Empty(next)
}

func (s *KeycapSetRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	sets, next, err := s.repo.List(context.Background(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().Error(err)
	s.Nil(sets)
	s.Empty(next)
}

func (s *KeycapSetRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "ks1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "ks1"},
				"brand":   &types.AttributeValueMemberS{Value: "GMK"},
			},
		}, nil)

	ks, err := s.repo.Get(context.Background(), "alice", "ks1")

	s.Require().NoError(err)
	s.Equal("ks1", ks.ID)
	s.Equal("GMK", ks.Brand)
}

func (s *KeycapSetRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ks, err := s.repo.Get(context.Background(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ks, err := s.repo.Get(context.Background(), "alice", "ks1")

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1", Brand: "GMK"})

	s.Require().NoError(err)
	s.Equal(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK"}, ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see errNoUserID (client.go).
	ks, err := s.repo.Create(context.Background(), repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			return *in.ConditionExpression == "attribute_exists(id)"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1", Brand: "GMK"})

	s.Require().NoError(err)
	s.Equal(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK"}, ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on PutItem - see errNoUserID (client.go).
	ks, err := s.repo.Update(context.Background(), repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	err := s.repo.Delete(ctx, "ks1")

	s.Require().NoError(err)
}

func (s *KeycapSetRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on DeleteItem - see errNoUserID (client.go).
	err := s.repo.Delete(context.Background(), "ks1")

	s.Require().Error(err)
}

func (s *KeycapSetRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	err := s.repo.Delete(ctx, "ks1")

	s.Require().Error(err)
}

func (s *KeycapSetRepositorySuite) getItemOutput(version int) *dynamodb.GetItemOutput {
	item := map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: "alice"},
		"id":      &types.AttributeValueMemberS{Value: "ks1"},
		"brand":   &types.AttributeValueMemberS{Value: "GMK"},
		"version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", version)},
	}
	return &dynamodb.GetItemOutput{Item: item}
}

func (s *KeycapSetRepositorySuite) TestAddKit_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			err := attributevalue.UnmarshalMap(in.Item, &ks)
			if err != nil {
				return false
			}
			return ks.Version == 1 && len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit1"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Equal("kit1", kit.KitID)
	s.Equal("Base", kit.Name)
}

func (s *KeycapSetRepositorySuite) TestAddKit_ParentSetNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	kit, err := s.repo.AddKit(ctx, "missing", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_CASConflict_RetriesThenSucceeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(1), nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			err := attributevalue.UnmarshalMap(in.Item, &ks)
			if err != nil {
				return false
			}
			return ks.Version == 2
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, errKitMutationExhausted)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(context.Background(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, errKitMutationExhausted)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - see errNoUserID (client.go).
	kit, err := s.repo.AddKit(context.Background(), "ks1", repository.KeycapKit{KitID: "kit1"})

	s.Require().Error(err)
	s.Nil(kit)
}
