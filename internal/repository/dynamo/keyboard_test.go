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
	// No EXPECT() on PutItem - see repository.ErrNoUserID.
	kb, err := s.repo.Create(s.T().Context(), repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var kb repository.Keyboard
			if err := attributevalue.UnmarshalMap(in.Item, &kb); err != nil {
				return false
			}
			return kb.Brand == "Keychron" && kb.Version == 1
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Equal("Keychron", kb.Brand)
}

func (s *KeyboardRepositorySuite) TestUpdate_PreservesExistingImagesAndVersion() {
	getOutput := s.getItemOutput(3)
	getOutput.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "keyboards/alice/kb1/images/img1"},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var kb repository.Keyboard
			if err := attributevalue.UnmarshalMap(in.Item, &kb); err != nil {
				return false
			}
			return kb.Version == 4 && len(kb.Images) == 1 && kb.Images[0].ImageID == "img1"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Require().Len(kb.Images, 1)
	s.Equal("img1", kb.Images[0].ImageID)
}

func (s *KeyboardRepositorySuite) TestUpdate_CASConflict_RetriesThenSucceeds() {
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img-from-winner"},
				"path":     &types.AttributeValueMemberS{Value: "keyboards/alice/kb1/images/img-from-winner"},
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
			var kb repository.Keyboard
			err := attributevalue.UnmarshalMap(in.Item, &kb)
			if err != nil {
				return false
			}
			return kb.Version == 2 && kb.Brand == "Keychron" &&
				len(kb.Images) == 1 && kb.Images[0].ImageID == "img-from-winner"
		})).
		Return(&dynamodb.PutItemOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().NoError(err)
	s.Require().NotNil(kb)
	s.Equal("Keychron", kb.Brand)
	s.Require().Len(kb.Images, 1)
	s.Equal("img-from-winner", kb.Images[0].ImageID)
}

func (s *KeyboardRepositorySuite) TestUpdate_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxKeyboardMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxKeyboardMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1", Brand: "Keychron"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	kb, err := s.repo.Update(ctx, repository.Keyboard{ID: "kb1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestUpdate_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
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
	// No EXPECT() on GetItem/PutItem - see repository.ErrNoUserID.
	kb, err := s.repo.Update(s.T().Context(), repository.Keyboard{ID: "kb1"})

	s.Require().Error(err)
	s.Nil(kb)
}

func (s *KeyboardRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{Attributes: s.getItemOutput(0).Item}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "kb1")

	s.Require().NoError(err)
	s.Empty(imageKeys)
}

func (s *KeyboardRepositorySuite) TestDelete_ImagesPresent_ReturnsTheirImageKeys() {
	out := s.getItemOutput(0)
	out.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "keyboards/alice/kb1/images/img1"},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img2"},
				"path":     &types.AttributeValueMemberS{Value: "keyboards/alice/kb1/images/img2"},
			}},
		},
	}

	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{Attributes: out.Item}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "kb1")

	s.Require().NoError(err)
	s.Require().Len(imageKeys, 2)
	s.Equal(repository.KeyboardImageKey("keyboards/alice/kb1/images/img1"), imageKeys[0])
	s.Equal(repository.KeyboardImageKey("keyboards/alice/kb1/images/img2"), imageKeys[1])
}

func (s *KeyboardRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on DeleteItem - see repository.ErrNoUserID.
	imageKeys, err := s.repo.Delete(s.T().Context(), "kb1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *KeyboardRepositorySuite) TestDelete_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "kb1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *KeyboardRepositorySuite) getItemOutput(version int) *dynamodb.GetItemOutput {
	item := map[string]types.AttributeValue{
		"user_id": &types.AttributeValueMemberS{Value: "alice"},
		"id":      &types.AttributeValueMemberS{Value: "kb1"},
		"brand":   &types.AttributeValueMemberS{Value: "Keychron"},
		"version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", version)},
	}
	return &dynamodb.GetItemOutput{Item: item}
}

func (s *KeyboardRepositorySuite) TestAddImage_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var kb repository.Keyboard
			err := attributevalue.UnmarshalMap(in.Item, &kb)
			if err != nil {
				return false
			}
			return kb.Version == 1 && len(kb.Images) == 1 && kb.Images[0].ImageID == "img1"
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "kb1", repository.KeyboardImage{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"})

	s.Require().NoError(err)
	s.Equal("img1", image.ImageID)
}

func (s *KeyboardRepositorySuite) TestAddImage_ParentKeyboardNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "kb1", repository.KeyboardImage{ImageID: "img1", Path: "p"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(image)
}

func (s *KeyboardRepositorySuite) TestAddImage_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxKeyboardMutationAttempts)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{}).Times(maxKeyboardMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "kb1", repository.KeyboardImage{ImageID: "img1", Path: "p"})

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(image)
}

func (s *KeyboardRepositorySuite) TestAddImage_EmptyImageID_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - caught before any repo call.
	image, err := s.repo.AddImage(kbdbctx.WithUserID(s.T().Context(), "alice"), "kb1", repository.KeyboardImage{Path: "p"})

	s.Require().Error(err)
	s.Nil(image)
}

func (s *KeyboardRepositorySuite) TestAddImage_DuplicateImageID_ReturnsError() {
	getOutput := s.getItemOutput(0)
	getOutput.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "p"},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "kb1", repository.KeyboardImage{ImageID: "img1", Path: "p2"})

	s.Require().Error(err)
	s.Nil(image)
}

func (s *KeyboardRepositorySuite) TestAddImage_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - see repository.ErrNoUserID.
	image, err := s.repo.AddImage(s.T().Context(), "kb1", repository.KeyboardImage{ImageID: "img1", Path: "p"})

	s.Require().Error(err)
	s.Nil(image)
}

func (s *KeyboardRepositorySuite) TestDeleteImage_Succeeds() {
	getOutput := s.getItemOutput(0)
	getOutput.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "keyboards/alice/kb1/images/img1"},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.PutItemInput) bool {
			var kb repository.Keyboard
			if err := attributevalue.UnmarshalMap(in.Item, &kb); err != nil {
				return false
			}
			return len(kb.Images) == 0
		})).
		Return(&dynamodb.PutItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "kb1", "img1")

	s.Require().NoError(err)
	s.Require().NotNil(removed)
	s.Equal(repository.KeyboardImageKey("keyboards/alice/kb1/images/img1"), *removed)
}

func (s *KeyboardRepositorySuite) TestDeleteImage_ImageAlreadyAbsent_SucceedsWithoutWriting() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	// No EXPECT() on PutItem - an absent image is a no-op, not a write.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "kb1", "missing")

	s.Require().NoError(err)
	s.Nil(removed)
}

func (s *KeyboardRepositorySuite) TestDeleteImage_ParentKeyboardNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "kb1", "img1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(removed)
}

func (s *KeyboardRepositorySuite) TestDeleteImage_NoUserIDInContext_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - see repository.ErrNoUserID.
	removed, err := s.repo.DeleteImage(s.T().Context(), "kb1", "img1")

	s.Require().Error(err)
	s.Nil(removed)
}
