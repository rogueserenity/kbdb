package dynamo

import (
	"errors"
	"fmt"
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

type BuildRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *BuildRepository
}

func TestBuildRepositorySuite(t *testing.T) {
	suite.Run(t, new(BuildRepositorySuite))
}

func (s *BuildRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &BuildRepository{client: s.mockClient, tableName: "build-table"}
}

func (s *BuildRepositorySuite) TestList_Succeeds() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			return len(in.ExclusiveStartKey) == 0 && *in.Limit == 20
		})).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{
				{
					"user_id":  &types.AttributeValueMemberS{Value: "alice"},
					"id":       &types.AttributeValueMemberS{Value: "b1"},
					"keyboard": &types.AttributeValueMemberS{Value: "kb1"},
				},
			},
		}, nil)

	builds, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.Empty(next)
	s.Require().Len(builds, 1)
	s.Equal("b1", builds[0].ID)
	s.Equal("kb1", builds[0].Keyboard)
}

func (s *BuildRepositorySuite) TestList_EmptyVisibilities_ReturnsEmptySliceWithoutQuerying() {
	// No EXPECT() on s.mockClient.Query - an empty visibilities slice must
	// short-circuit before building a Query, since expression.In(...)
	// requires at least one value and would otherwise panic.
	builds, next, err := s.repo.List(s.T().Context(), "alice", nil, 20, "")

	s.Require().NoError(err)
	s.NotNil(builds)
	s.Empty(builds)
	s.Empty(next)
}

func (s *BuildRepositorySuite) TestList_EmptyResult_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil)

	builds, _, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotNil(builds)
	s.Empty(builds)
}

func (s *BuildRepositorySuite) TestList_ReturnsEncodedCursor_WhenMorePagesExist() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(&dynamodb.QueryOutput{
			Items: []map[string]types.AttributeValue{},
			LastEvaluatedKey: map[string]types.AttributeValue{
				"user_id": &types.AttributeValueMemberS{Value: "alice"},
				"id":      &types.AttributeValueMemberS{Value: "b1"},
			},
		}, nil)

	_, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().NoError(err)
	s.NotEmpty(next)
}

func (s *BuildRepositorySuite) TestList_DecodesCursor_IntoExclusiveStartKey() {
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
				"id":      &types.AttributeValueMemberS{Value: "b1"},
			},
		}, nil).Once()

	_, cursor, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")
	s.Require().NoError(err)

	s.mockClient.EXPECT().
		Query(mock.Anything, mock.MatchedBy(func(in *dynamodb.QueryInput) bool {
			key, ok := in.ExclusiveStartKey["id"].(*types.AttributeValueMemberS)
			return ok && key.Value == "b1"
		})).
		Return(&dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{}}, nil).Once()

	_, _, err = s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, cursor)
	s.Require().NoError(err)
}

func (s *BuildRepositorySuite) TestList_InvalidCursor_ReturnsError() {
	builds, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().Error(err)
	s.Nil(builds)
	s.Empty(next)
}

func (s *BuildRepositorySuite) TestList_QueryError_Propagates() {
	s.mockClient.EXPECT().
		Query(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	builds, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "")

	s.Require().Error(err)
	s.Nil(builds)
	s.Empty(next)
}

func (s *BuildRepositorySuite) TestGet_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			userID, ok := in.Key["user_id"].(*types.AttributeValueMemberS)
			id, ok2 := in.Key["id"].(*types.AttributeValueMemberS)
			return ok && ok2 && userID.Value == "alice" && id.Value == "b1"
		})).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"user_id":  &types.AttributeValueMemberS{Value: "alice"},
				"id":       &types.AttributeValueMemberS{Value: "b1"},
				"keyboard": &types.AttributeValueMemberS{Value: "kb1"},
			},
		}, nil)

	b, err := s.repo.Get(s.T().Context(), "alice", "b1")

	s.Require().NoError(err)
	s.Equal("b1", b.ID)
	s.Equal("kb1", b.Keyboard)
}

func (s *BuildRepositorySuite) TestGet_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	b, err := s.repo.Get(s.T().Context(), "alice", "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestGet_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	b, err := s.repo.Get(s.T().Context(), "alice", "b1")

	s.Require().Error(err)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestCreate_Succeeds() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			// One item for the base Build, one marker for its keyboard.
			return len(in.TransactItems) == 2 && in.TransactItems[0].Put != nil
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Create(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})

	s.Require().NoError(err)
	s.Equal(&repository.Build{UserID: "alice", ID: "b1", Keyboard: "kb1"}, b)
}

func (s *BuildRepositorySuite) TestCreate_NoReferences_WritesOnlyBaseItem() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			return len(in.TransactItems) == 1
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Create(ctx, repository.Build{ID: "b1"})

	s.Require().NoError(err)
	s.Equal("b1", b.ID)
}

func (s *BuildRepositorySuite) TestCreate_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		})

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Create(ctx, repository.Build{ID: "b1"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestCreate_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Create(ctx, repository.Build{ID: "b1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestCreate_NoUserIDInContext_ReturnsError() {
	b, err := s.repo.Create(s.T().Context(), repository.Build{ID: "b1"})

	s.Require().Error(err)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			if err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b); err != nil {
				return false
			}
			return b.Keyboard == "kb2" && b.Version == 1
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb2"})

	s.Require().NoError(err)
	s.Equal("kb2", b.Keyboard)
}

func (s *BuildRepositorySuite) TestUpdate_ReferenceChange_DiffsMarkers() {
	// Existing build (via getItemOutput) references kb1; updating to kb2
	// must add a kb2 marker and remove the kb1 marker, alongside the base
	// item's own CAS-conditioned Put - three transact items total.
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			if len(in.TransactItems) != 3 {
				return false
			}
			var addsKb2, removesKb1 bool
			for _, item := range in.TransactItems {
				if item.Put != nil {
					if refID, ok := item.Put.Item["ref_id"].(*types.AttributeValueMemberS); ok && refID.Value == "kb2" {
						addsKb2 = true
					}
				}
				if item.Delete != nil {
					if id, ok := item.Delete.Key["id"].(*types.AttributeValueMemberS); ok && id.Value == "zREF#keyboard#kb1#b1" {
						removesKb1 = true
					}
				}
			}
			return addsKb2 && removesKb1
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb2"})

	s.Require().NoError(err)
	s.Equal("kb2", b.Keyboard)
}

func (s *BuildRepositorySuite) TestUpdate_PreservesExistingImagesAndVersion() {
	getOutput := s.getItemOutput(3)
	getOutput.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img1"},
			}},
		},
	}
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			if err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b); err != nil {
				return false
			}
			return b.Version == 4 && len(b.Images) == 1 && b.Images[0].ImageID == "img1"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb2"})

	s.Require().NoError(err)
	s.Require().Len(b.Images, 1)
	s.Equal("img1", b.Images[0].ImageID)
}

func (s *BuildRepositorySuite) TestUpdate_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a build with an image (image-from-winner) that
	// the first Get never saw - proves the retry re-reads fresh state
	// rather than overlaying onto the first attempt's now-stale struct.
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "image-from-winner"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/image-from-winner"},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b)
			if err != nil {
				return false
			}
			return b.Version == 2 && b.Keyboard == "kb2" &&
				len(b.Images) == 1 && b.Images[0].ImageID == "image-from-winner"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb2"})

	s.Require().NoError(err)
	s.Require().NotNil(b)
	s.Equal("kb2", b.Keyboard)
	s.Require().Len(b.Images, 1)
	s.Equal("image-from-winner", b.Images[0].ImageID)
}

func (s *BuildRepositorySuite) TestUpdate_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxBuildMutationAttempts)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Times(maxBuildMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb2"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_NoUserIDInContext_ReturnsError() {
	b, err := s.repo.Update(s.T().Context(), repository.Build{ID: "b1"})

	s.Require().Error(err)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		// One item for the base Build, one marker delete for its keyboard.
		// The base item's delete is CAS-conditioned on the version this Get
		// just saw, so a concurrent Update can't leave an orphaned marker.
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			return len(in.TransactItems) == 2 && in.TransactItems[0].Delete != nil &&
				in.TransactItems[0].Delete.ConditionExpression != nil
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
	s.Empty(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_ConcurrentUpdate_RetriesThenSucceeds() {
	// The second Get sees a build referencing kb2 instead of kb1 - proves
	// the retry recomputes markers from fresh state (kb2's marker deleted,
	// not kb1's stale one) rather than reusing the first Get's snapshot.
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["keyboard"] = &types.AttributeValueMemberS{Value: "kb2"}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			if len(in.TransactItems) != 2 || in.TransactItems[1].Delete == nil {
				return false
			}
			id, ok := in.TransactItems[1].Delete.Key["id"].(*types.AttributeValueMemberS)
			return ok && id.Value == "zREF#keyboard#kb2#b1"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
	s.Empty(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxBuildMutationAttempts)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Times(maxBuildMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_ImagesPresent_ReturnsTheirImageKeys() {
	out := s.getItemOutput(0)
	out.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img1"},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img2"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img2"},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(out, nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
	s.Require().Len(imageKeys, 2)
	s.Equal(repository.BuildImageKey("builds/alice/b1/images/img1"), imageKeys[0])
	s.Equal(repository.BuildImageKey("builds/alice/b1/images/img2"), imageKeys[1])
}

func (s *BuildRepositorySuite) TestDelete_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)
	// No EXPECT() on TransactWriteItems - a missing build is caught by the
	// Get before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	imageKeys, err := s.repo.Delete(s.T().Context(), "b1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *BuildRepositorySuite) TestDelete_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	imageKeys, err := s.repo.Delete(ctx, "b1")

	s.Require().Error(err)
	s.Nil(imageKeys)
}

func (s *BuildRepositorySuite) getItemOutput(version int) *dynamodb.GetItemOutput {
	item := map[string]types.AttributeValue{
		"user_id":  &types.AttributeValueMemberS{Value: "alice"},
		"id":       &types.AttributeValueMemberS{Value: "b1"},
		"keyboard": &types.AttributeValueMemberS{Value: "kb1"},
		"version":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", version)},
	}
	return &dynamodb.GetItemOutput{Item: item}
}

// getItemOutputWithImage returns a build (img1) for the DeleteImage tests
// that need a target image already present to remove.
func (s *BuildRepositorySuite) getItemOutputWithImage() *dynamodb.GetItemOutput {
	out := s.getItemOutput(0)
	out.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img1"},
			}},
		},
	}
	return out
}

func (s *BuildRepositorySuite) TestAddImage_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b)
			if err != nil {
				return false
			}
			return b.Version == 1 && len(b.Images) == 1 && b.Images[0].ImageID == "img1"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{
		ImageID: "img1",
		Path:    "builds/alice/b1/images/img1",
	})

	s.Require().NoError(err)
	s.Require().NotNil(image)
	s.Equal("img1", image.ImageID)
}

func (s *BuildRepositorySuite) TestAddImage_ParentBuildNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "missing", repository.BuildImage{ImageID: "img1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestAddImage_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a build with an image (image-from-winner) that
	// the first Get never saw - proves the retry re-reads fresh state and
	// mutates that, rather than re-applying the mutation to the first
	// attempt's now-stale in-memory struct.
	firstGet := s.getItemOutput(0)
	secondGet := s.getItemOutput(1)
	secondGet.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "image-from-winner"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/image-from-winner"},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b)
			if err != nil {
				return false
			}
			return b.Version == 2 && len(b.Images) == 2 &&
				b.Images[0].ImageID == "image-from-winner" && b.Images[1].ImageID == "img1"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{
		ImageID: "img1",
		Path:    "builds/alice/b1/images/img1",
	})

	s.Require().NoError(err)
	s.Require().NotNil(image)
	s.Equal("img1", image.ImageID)
}

func (s *BuildRepositorySuite) TestAddImage_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil).Times(maxBuildMutationAttempts)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Times(maxBuildMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1"})

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestAddImage_EmptyImageID_ReturnsError() {
	// No EXPECT() on GetItem/PutItem - caught before any DynamoDB call.
	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{})

	s.Require().ErrorIs(err, errEmptyImageID)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestAddImage_DuplicateImageID_ReturnsError() {
	getOutput := s.getItemOutputWithImage()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(getOutput, nil)
	// No EXPECT() on PutItem - the duplicate is caught inside the mutate
	// closure, before any write is attempted.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{
		ImageID: "img1",
		Path:    "builds/alice/b1/images/img1",
	})

	s.Require().ErrorIs(err, errDuplicateImageID)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestAddImage_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	image, err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestAddImage_NoUserIDInContext_ReturnsError() {
	image, err := s.repo.AddImage(s.T().Context(), "b1", repository.BuildImage{ImageID: "img1"})

	s.Require().Error(err)
	s.Nil(image)
}

func (s *BuildRepositorySuite) TestDeleteImage_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithImage(), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			if err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b); err != nil {
				return false
			}
			return b.Version == 1 && len(b.Images) == 0
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().NoError(err)
	s.Require().NotNil(removed)
	s.Equal(repository.BuildImageKey("builds/alice/b1/images/img1"), *removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_ImageAlreadyAbsent_SucceedsWithoutWriting() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutput(0), nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "no-such-image")

	s.Require().NoError(err)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_ParentBuildNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "missing", "img1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_CASConflict_RetriesThenSucceeds() {
	// The second Get returns a build with an additional image
	// (image-from-winner) alongside img1 - proves the retry recomputes
	// state from a fresh Get rather than reusing the first attempt's
	// now-stale slice.
	firstGet := s.getItemOutputWithImage()
	secondGet := s.getItemOutput(1)
	secondGet.Item["images"] = &types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "image-from-winner"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/image-from-winner"},
			}},
			&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"image_id": &types.AttributeValueMemberS{Value: "img1"},
				"path":     &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img1"},
			}},
		},
	}

	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(firstGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Once()
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(secondGet, nil).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			var b repository.Build
			if err := attributevalue.UnmarshalMap(in.TransactItems[0].Put.Item, &b); err != nil {
				return false
			}
			return b.Version == 2 && len(b.Images) == 1 && b.Images[0].ImageID == "image-from-winner"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().NoError(err)
	s.Require().NotNil(removed)
	s.Equal(repository.BuildImageKey("builds/alice/b1/images/img1"), *removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_CASConflictExhausted_ReturnsError() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithImage(), nil).Times(maxBuildMutationAttempts)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, &types.TransactionCanceledException{
			CancellationReasons: []types.CancellationReason{{Code: aws.String("ConditionalCheckFailed")}},
		}).Times(maxBuildMutationAttempts)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().Error(err)
	s.Require().ErrorIs(err, repository.ErrMutationConflict)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.getItemOutputWithImage(), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrMutationConflict)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_NoUserIDInContext_ReturnsError() {
	removed, err := s.repo.DeleteImage(s.T().Context(), "b1", "img1")

	s.Require().Error(err)
	s.Nil(removed)
}
