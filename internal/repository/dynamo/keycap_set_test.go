package dynamo

import (
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

	sets, next, err := s.repo.List(s.T().Context(), "alice",
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
	sets, next, err := s.repo.List(s.T().Context(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(sets)
	s.Empty(sets)
	s.Empty(next)
}

func (s *KeycapSetRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	sets, _, err := s.repo.List(s.T().Context(), "alice",
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

	_, next, err := s.repo.List(s.T().Context(), "alice",
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

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "ks1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *KeycapSetRepositorySuite) TestList_InvalidCursor_ReturnsError() {
	sets, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().Error(err)
	s.Nil(sets)
	s.Empty(next)
}

func (s *KeycapSetRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	sets, next, err := s.repo.List(s.T().Context(), "alice",
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

	ks, err := s.repo.Get(s.T().Context(), "alice", "ks1")

	s.Require().NoError(err)
	s.Equal("ks1", ks.ID)
	s.Equal("GMK", ks.Brand)
}

func (s *KeycapSetRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ks, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ks, err := s.repo.Get(s.T().Context(), "alice", "ks1")

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1", Brand: "GMK"})

	s.Require().NoError(err)
	s.Equal(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK"}, ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Create(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	ks, err := s.repo.Create(s.T().Context(), repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Brand == "Keychron" && ks.Version == 1
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Equal("Keychron", ks.Brand)
}

func (s *KeycapSetRepositorySuite) TestUpdate_PreservesExistingKitsAndVersion() {
	getOutput := s.getItemOutput(3)
	getOutput.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 4 && len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit1"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Require().Len(ks.Kits, 1)
	s.Equal("kit1", ks.Kits[0].KitID)
}

func (s *KeycapSetRepositorySuite) TestUpdate_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with a kit (kit-from-winner) that the
	// first Get never saw - proves the retry re-reads fresh state rather
	// than overlaying onto the first attempt's now-stale struct.
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			err := attributevalue.UnmarshalMap(in.Item, &ks)
			if err != nil {
				return false
			}
			return ks.Version == 2 && ks.Brand == "Keychron" &&
				len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit-from-winner"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Require().NotNil(ks)
	s.Equal("Keychron", ks.Brand)
	s.Require().Len(ks.Kits, 1)
	s.Equal("kit-from-winner", ks.Kits[0].KitID)
}

func (s *KeycapSetRepositorySuite) TestUpdate_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1", Brand: "Keychron"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	ks, err := s.repo.Update(ctx, repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	ks, err := s.repo.Update(s.T().Context(), repository.KeycapSet{ID: "ks1"})

	s.Require().Error(err)
	s.Nil(ks)
}

func (s *KeycapSetRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{Attributes: s.getItemOutput(0).Item}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "ks1")

	s.Require().NoError(err)
	s.Empty(imageKeys)
}

func (s *KeycapSetRepositorySuite) TestDelete_KitsWithImages_ReturnsTheirImageKeys() {
	out := s.getItemOutput(0)
	out.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":     &types.AttributeValueMemberS{Value: "kit1"},
				"name":       &types.AttributeValueMemberS{Value: "Base"},
				"image_path": &types.AttributeValueMemberS{Value: "keycap-sets/alice/ks1/kits/kit1/image"},
				"purchase":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit2"},
				"name":     &types.AttributeValueMemberS{Value: "Extension"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":     &types.AttributeValueMemberS{Value: "kit3"},
				"name":       &types.AttributeValueMemberS{Value: "Novelties"},
				"image_path": &types.AttributeValueMemberS{Value: "keycap-sets/alice/ks1/kits/kit3/image"},
				"purchase":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{Attributes: out.Item}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "ks1")

	s.Require().NoError(err)
	s.Require().Len(imageKeys, 2)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), imageKeys[0])
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit3/image"), imageKeys[1])
}

func (s *KeycapSetRepositorySuite) TestDelete_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "ks1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(imageKeys)
}

func (s *KeycapSetRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	imageKeys, err := s.repo.Delete(s.T().Context(), "ks1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *KeycapSetRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "ks1")

	s.Require().Error(err)
	s.Nil(imageKeys)
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

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
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

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "missing", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with a kit (kit-from-winner) that the
	// first Get never saw - proves the retry re-reads fresh state and
	// mutates that, rather than re-applying the mutation to the first
	// attempt's now-stale in-memory struct (which would silently drop
	// kit-from-winner).
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			err := attributevalue.UnmarshalMap(in.Item, &ks)
			if err != nil {
				return false
			}
			return ks.Version == 2 && len(ks.Kits) == 2 &&
				ks.Kits[0].KitID == "kit-from-winner" && ks.Kits[1].KitID == "kit1"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Equal("kit1", kit.KitID)
	s.Equal("Base", kit.Name)
}

func (s *KeycapSetRepositorySuite) TestAddKit_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_EmptyKitID_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - caught before any DynamoDB call.
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{Name: "Base"})

	s.Require().ErrorIs(err, errEmptyKitID)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_DuplicateKitID_ReturnsError() {
	getOutput := s.getItemOutput(0)
	getOutput.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	// No EXPECT() on PutItem - the duplicate is caught inside the mutate
	// closure, before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().ErrorIs(err, errDuplicateKitID)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.AddKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Base"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestAddKit_NoUserIDInContext_ReturnsError() {
	kit, err := s.repo.AddKit(s.T().Context(), "ks1", repository.KeycapKit{KitID: "kit1"})

	s.Require().Error(err)
	s.Nil(kit)
}

// getItemOutputWithKit returns a set (kit1/"Base") for the UpdateKit tests
// that need a target kit already present to update.
func (s *KeycapSetRepositorySuite) getItemOutputWithKit() *dynamodb.GetItemOutput {
	out := s.getItemOutput(0)
	out.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}
	return out
}

func (s *KeycapSetRepositorySuite) getItemOutputWithKitImage() *dynamodb.GetItemOutput {
	out := s.getItemOutput(0)
	out.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":     &types.AttributeValueMemberS{Value: "kit1"},
				"name":       &types.AttributeValueMemberS{Value: "Base"},
				"image_path": &types.AttributeValueMemberS{Value: "keycap-sets/alice/ks1/kits/kit1/image"},
				"purchase":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}
	return out
}

func (s *KeycapSetRepositorySuite) getItemOutputWithPrimaryKit(primaryKitID string) *dynamodb.GetItemOutput {
	out := s.getItemOutputWithKit()
	out.Item["primary_kit_id"] = &types.AttributeValueMemberS{Value: primaryKitID}
	return out
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_KitIsPrimary_ClearsPrimaryKitID() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithPrimaryKit("kit1"), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return len(ks.Kits) == 0 && ks.PrimaryKitID == nil
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().NoError(err)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_KitIsNotPrimary_LeavesPrimaryKitIDUntouched() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithPrimaryKit("some-other-kit"), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return len(ks.Kits) == 0 && ks.PrimaryKitID != nil && *ks.PrimaryKitID == "some-other-kit"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().NoError(err)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 1 && len(ks.Kits) == 1 &&
				ks.Kits[0].KitID == "kit1" && ks.Kits[0].Name == "Extension" &&
				ks.Kits[0].Purchase.Vendor != nil && *ks.Kits[0].Purchase.Vendor == "CannonKeys"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	vendor := "CannonKeys"
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{
		KitID:    "kit1",
		Name:     "Extension",
		Purchase: repository.KeycapKitPurchase{Vendor: &vendor},
	})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Equal("kit1", kit.KitID)
	s.Equal("Extension", kit.Name)
	s.Require().NotNil(kit.Purchase.Vendor)
	s.Equal("CannonKeys", *kit.Purchase.Vendor)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_PreservesImagePath() {
	getOutput := s.getItemOutput(0)
	getOutput.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":     &types.AttributeValueMemberS{Value: "kit1"},
				"name":       &types.AttributeValueMemberS{Value: "Base"},
				"image_path": &types.AttributeValueMemberS{Value: "keycap-sets/alice/ks1/kits/kit1/image"},
				"purchase":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return len(ks.Kits) == 1 && ks.Kits[0].Name == "Extension" &&
				ks.Kits[0].ImagePath != nil && *ks.Kits[0].ImagePath == "keycap-sets/alice/ks1/kits/kit1/image"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Require().NotNil(kit.ImagePath)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *kit.ImagePath)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_KitNotFoundInExistingSet_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	// No EXPECT() on PutItem - a missing kit is caught inside the mutate
	// closure, before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{KitID: "missing-kit", Name: "Extension"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_ParentSetNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "missing", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with a different kit (kit-from-winner)
	// placed BEFORE kit1, shifting kit1 from index 0 to index 1 - proves
	// the retry recomputes which index to update against fresh state,
	// rather than reusing an index captured from the first attempt (which
	// would wrongly update kit-from-winner here).
	firstGet := s.getItemOutputWithKit()
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 2 && len(ks.Kits) == 2 &&
				ks.Kits[0].KitID == "kit-from-winner" && ks.Kits[0].Name == "Winner" &&
				ks.Kits[1].KitID == "kit1" && ks.Kits[1].Name == "Extension"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Equal("Extension", kit.Name)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_EmptyKitID_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - caught before any DynamoDB call.
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{Name: "Extension"})

	s.Require().ErrorIs(err, errEmptyKitID)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.UpdateKit(ctx, "ks1", repository.KeycapKit{KitID: "kit1", Name: "Extension"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestUpdateKit_NoUserIDInContext_ReturnsError() {
	kit, err := s.repo.UpdateKit(s.T().Context(), "ks1", repository.KeycapKit{KitID: "kit1"})

	s.Require().Error(err)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKitImage(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 1 && len(ks.Kits) == 0
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_KitAlreadyAbsent_SucceedsWithoutWriting() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "ks1", "no-such-kit")

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_ParentSetNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "missing", "kit1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with an additional kit (kit-from-winner)
	// alongside kit1 - proves the retry recomputes state from a fresh Get
	// rather than reusing the first attempt's now-stale slice.
	firstGet := s.getItemOutputWithKit()
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 2 && len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit-from-winner"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.DeleteKit(ctx, "ks1", "kit1")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestDeleteKit_NoUserIDInContext_ReturnsError() {
	cleared, err := s.repo.DeleteKit(s.T().Context(), "ks1", "kit1")

	s.Require().Error(err)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 1 && len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit1" &&
				ks.Kits[0].ImagePath != nil && *ks.Kits[0].ImagePath == "keycap-sets/alice/ks1/kits/kit1/image"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "ks1", "kit1", "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Require().NotNil(kit.ImagePath)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *kit.ImagePath)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_KitNotFoundInExistingSet_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	// No EXPECT() on PutItem - a missing kit is caught inside the mutate
	// closure, before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "ks1", "missing-kit", "keycap-sets/alice/ks1/kits/missing-kit/image")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_ParentSetNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "missing", "kit1", "keycap-sets/alice/missing/kits/kit1/image")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with a different kit (kit-from-winner)
	// placed BEFORE kit1, shifting kit1 from index 0 to index 1 - proves
	// the retry recomputes which index to update against fresh state.
	firstGet := s.getItemOutputWithKit()
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit1"},
				"name":     &types.AttributeValueMemberS{Value: "Base"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 2 && len(ks.Kits) == 2 &&
				ks.Kits[0].KitID == "kit-from-winner" &&
				ks.Kits[1].KitID == "kit1" && ks.Kits[1].ImagePath != nil &&
				*ks.Kits[1].ImagePath == "keycap-sets/alice/ks1/kits/kit1/image"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "ks1", "kit1", "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().NoError(err)
	s.Require().NotNil(kit)
	s.Require().NotNil(kit.ImagePath)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *kit.ImagePath)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "ks1", "kit1", "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kit, err := s.repo.SetKitImagePath(ctx, "ks1", "kit1", "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestSetKitImagePath_NoUserIDInContext_ReturnsError() {
	kit, err := s.repo.SetKitImagePath(s.T().Context(), "ks1", "kit1", "keycap-sets/alice/ks1/kits/kit1/image")

	s.Require().Error(err)
	s.Nil(kit)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKitImage(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 1 && len(ks.Kits) == 1 && ks.Kits[0].KitID == "kit1" &&
				ks.Kits[0].ImagePath == nil
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_AlreadyAbsent_NoWrite() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKit(), nil)
	// No EXPECT() on PutItem - errKitImageAlreadyAbsent short-circuits
	// mutateSet before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_KitNotFoundInExistingSet_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKitImage(), nil)
	// No EXPECT() on PutItem - a missing kit is caught inside the mutate
	// closure, before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "missing-kit")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_ParentSetNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "missing", "kit1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a set with a different kit (kit-from-winner)
	// placed BEFORE kit1, shifting kit1 from index 0 to index 1 - proves
	// the retry recomputes which index to update against fresh state.
	firstGet := s.getItemOutputWithKitImage()
	secondGet := s.getItemOutput(1)
	secondGet.Item["kits"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":   &types.AttributeValueMemberS{Value: "kit-from-winner"},
				"name":     &types.AttributeValueMemberS{Value: "Winner"},
				"purchase": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"kit_id":     &types.AttributeValueMemberS{Value: "kit1"},
				"name":       &types.AttributeValueMemberS{Value: "Base"},
				"image_path": &types.AttributeValueMemberS{Value: "keycap-sets/alice/ks1/kits/kit1/image"},
				"purchase":   &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{}},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var ks repository.KeycapSet
			if err := attributevalue.UnmarshalMap(in.Item, &ks); err != nil {
				return false
			}
			return ks.Version == 2 && len(ks.Kits) == 2 &&
				ks.Kits[0].KitID == "kit-from-winner" &&
				ks.Kits[1].KitID == "kit1" && ks.Kits[1].ImagePath == nil
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "kit1")

	s.Require().NoError(err)
	s.Require().NotNil(cleared)
	s.Equal(repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image"), *cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKitImage(), nil).Times(maxSetMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxSetMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "kit1")

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithKitImage(), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	cleared, err := s.repo.ClearKitImagePath(ctx, "ks1", "kit1")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(cleared)
}

func (s *KeycapSetRepositorySuite) TestClearKitImagePath_NoUserIDInContext_ReturnsError() {
	cleared, err := s.repo.ClearKitImagePath(s.T().Context(), "ks1", "kit1")

	s.Require().Error(err)
	s.Nil(cleared)
}
