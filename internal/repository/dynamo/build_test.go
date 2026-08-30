package dynamo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func (s *BuildRepositorySuite) TestList_InvalidCursor_ReturnsErrInvalidCursor() {
	builds, next, err := s.repo.List(s.T().Context(), "alice",
		[]repository.Visibility{repository.VisibilityPublic}, 20, "not-valid-base64!!")

	s.Require().ErrorIs(err, repository.ErrInvalidCursor)
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

func (s *BuildRepositorySuite) TestCreate_TransactionConflict_ReturnsErrMutationConflict() {
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("TransactionConflict"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Create(ctx, repository.Build{ID: "b1"})

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
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


// storedBuild is a GetItemOutput for build b1 referencing keyboard kb1,
// optionally with the given image id->seq pairs in its images map.
func (s *BuildRepositorySuite) storedBuild(seqByImageID map[string]int) *dynamodb.GetItemOutput {
	item := map[string]types.AttributeValue{
		"user_id":  &types.AttributeValueMemberS{Value: "alice"},
		"id":       &types.AttributeValueMemberS{Value: "b1"},
		"keyboard": &types.AttributeValueMemberS{Value: "kb1"},
	}
	if seqByImageID != nil {
		imgs := map[string]types.AttributeValue{}
		for id, seq := range seqByImageID {
			imgs[id] = &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"path": &types.AttributeValueMemberS{Value: "builds/alice/b1/images/" + id},
				"seq":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", seq)},
			}}
		}
		item["images"] = &types.AttributeValueMemberM{Value: imgs}
	}
	return &dynamodb.GetItemOutput{Item: item}
}

func txCanceled(codes ...string) *types.TransactionCanceledException {
	reasons := make([]types.CancellationReason, len(codes))
	for i, c := range codes {
		reasons[i] = types.CancellationReason{Code: aws.String(c)}
	}
	return &types.TransactionCanceledException{CancellationReasons: reasons}
}

func (s *BuildRepositorySuite) TestUpdate_Succeeds() {
	// Get (for the marker diff), the transaction, then a Get to return the
	// persisted build.
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil).Twice()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			// Item 0 is an Update action on the build item, not a Put.
			return in.TransactItems[0].Update != nil && in.TransactItems[0].Put == nil &&
				strings.Contains(*in.TransactItems[0].Update.ConditionExpression, "attribute_exists")
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})

	s.Require().NoError(err)
	s.Equal("kb1", b.Keyboard)
}

func (s *BuildRepositorySuite) TestUpdate_ReferenceChange_DiffsMarkers() {
	// Existing build references kb1; updating to kb2 adds a kb2 marker and
	// removes the kb1 marker, alongside the build item's Update action.
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil).Once() // diff-get: still kb1
	kb2 := s.storedBuild(nil)
	kb2.Item["keyboard"] = &types.AttributeValueMemberS{Value: "kb2"}
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(kb2, nil).Once() // return-get: reflects the committed kb2
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			if len(in.TransactItems) != 3 || in.TransactItems[0].Update == nil {
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

func (s *BuildRepositorySuite) TestUpdate_DoesNotNameImages_SoTheyCarryForward() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(map[string]int{"img1": 0}), nil).Once()
	var captured *dynamodb.TransactWriteItemsInput
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			captured = in
			return true
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)
	// Return-read shows img1 still present.
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(map[string]int{"img1": 0}), nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})
	s.Require().NoError(err)

	expr := *captured.TransactItems[0].Update.UpdateExpression
	s.NotContains(expr, "images")
	s.Len(b.Images, 1)
}

func (s *BuildRepositorySuite) TestUpdate_BuildGone_ReturnsErrNotFound() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("ConditionalCheckFailed"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_TransactionConflict_RetriesThenSucceeds() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil).Times(3) // diff-get x2 + final return-get
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("TransactionConflict")).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})

	s.Require().NoError(err)
	s.Equal("kb1", b.Keyboard)
}

func (s *BuildRepositorySuite) TestUpdate_ConflictExhaustsRetries_ReturnsErrMutationConflict() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("TransactionConflict"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	_, err := s.repo.Update(ctx, repository.Build{ID: "b1", Keyboard: "kb1"})

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
}

func (s *BuildRepositorySuite) TestUpdate_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	b, err := s.repo.Update(ctx, repository.Build{ID: "b1"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(b)
}

func (s *BuildRepositorySuite) TestUpdate_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
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

func (s *BuildRepositorySuite) TestDelete_Succeeds_NoVersionConditionOnBuildItem() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			// The build-item Delete carries no condition now.
			return in.TransactItems[0].Delete != nil &&
				in.TransactItems[0].Delete.ConditionExpression == nil
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
}

func (s *BuildRepositorySuite) TestDelete_DeletesRefMarkers() {
	// storedBuild references kb1, so the transaction includes the build
	// Delete plus a kb1 marker Delete.
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.MatchedBy(func(in *dynamodb.TransactWriteItemsInput) bool {
			if len(in.TransactItems) != 2 {
				return false
			}
			id, ok := in.TransactItems[1].Delete.Key["id"].(*types.AttributeValueMemberS)
			return ok && id.Value == "zREF#keyboard#kb1#b1"
		})).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
}

func (s *BuildRepositorySuite) TestDelete_TransactionConflict_RetriesThenSucceeds() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil).Twice()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("TransactionConflict")).Once()
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(&dynamodb.TransactWriteItemsOutput{}, nil).Once()

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
}

func (s *BuildRepositorySuite) TestDelete_ConflictExhaustsRetries_ReturnsErrMutationConflict() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, txCanceled("TransactionConflict"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().ErrorIs(err, repository.ErrMutationConflict)
}

func (s *BuildRepositorySuite) TestDelete_NotFound_IsNoOpSuccess() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)
	// No TransactWriteItems - nothing to delete.

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().NoError(err)
}

func (s *BuildRepositorySuite) TestDelete_NoUserIDInContext_ReturnsError() {
	err := s.repo.Delete(s.T().Context(), "b1")

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestDelete_GetItemError_Propagates() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestDelete_TransactWriteItemsError_Propagates() {
	s.mockClient.EXPECT().GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)
	s.mockClient.EXPECT().
		TransactWriteItems(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.Delete(ctx, "b1")

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestAddImage_Succeeds_PlainUpdateItemNoTransaction() {
	// One UpdateItem, no GetItem, no transaction. seq is a wall-clock stamp.
	before := time.Now().UnixNano()
	var captured *dynamodb.UpdateItemInput
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			captured = in
			return strings.Contains(*in.ConditionExpression, "attribute_exists") &&
				strings.Contains(*in.ConditionExpression, "attribute_not_exists")
		})).
		Return(&dynamodb.UpdateItemOutput{}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1", Path: "builds/alice/b1/images/img1"})
	s.Require().NoError(err)

	entry := captured.ExpressionAttributeValues[":0"].(*types.AttributeValueMemberM)
	seq, convErr := strconv.ParseInt(entry.Value["seq"].(*types.AttributeValueMemberN).Value, 10, 64)
	s.Require().NoError(convErr)
	s.GreaterOrEqual(seq, before)
	s.LessOrEqual(seq, time.Now().UnixNano())
}

func (s *BuildRepositorySuite) TestAddImage_ParentBuildNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.GetItemInput) bool {
			return in.ConsistentRead != nil && *in.ConsistentRead
		})).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1", Path: "p"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
}

func (s *BuildRepositorySuite) TestAddImage_DuplicateImageID_ReturnsError() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1", Path: "p2"})

	s.Require().ErrorIs(err, errDuplicateImageID)
}

func (s *BuildRepositorySuite) TestAddImage_ClassifyGetItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1", Path: "p"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
}

func (s *BuildRepositorySuite) TestAddImage_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	err := s.repo.AddImage(ctx, "b1", repository.BuildImage{ImageID: "img1", Path: "p"})

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestAddImage_EmptyImageID_ReturnsError() {
	err := s.repo.AddImage(kbdbctx.WithUserID(s.T().Context(), "alice"), "b1", repository.BuildImage{Path: "p"})

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestAddImage_NoUserIDInContext_ReturnsError() {
	err := s.repo.AddImage(s.T().Context(), "b1", repository.BuildImage{ImageID: "img1", Path: "p"})

	s.Require().Error(err)
}

func (s *BuildRepositorySuite) TestDeleteImage_Succeeds() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.MatchedBy(func(in *dynamodb.UpdateItemInput) bool {
			return strings.Contains(*in.UpdateExpression, "REMOVE") &&
				in.ReturnValues == types.ReturnValueAllOld
		})).
		Return(&dynamodb.UpdateItemOutput{Attributes: map[string]types.AttributeValue{
			"images": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
				"img1": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"path": &types.AttributeValueMemberS{Value: "builds/alice/b1/images/img1"},
					"seq":  &types.AttributeValueMemberN{Value: "0"},
				}},
			}},
		}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().NoError(err)
	s.Require().NotNil(removed)
	s.Equal(repository.BuildImageKey("builds/alice/b1/images/img1"), *removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_ImageAlreadyAbsent_ReturnsNilWithoutError() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(s.storedBuild(nil), nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "missing")

	s.Require().NoError(err)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_ParentBuildNotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_UpdateItemError_Propagates() {
	s.mockClient.EXPECT().
		UpdateItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	ctx := kbdbctx.WithUserID(s.T().Context(), "alice")
	removed, err := s.repo.DeleteImage(ctx, "b1", "img1")

	s.Require().Error(err)
	s.Nil(removed)
}

func (s *BuildRepositorySuite) TestDeleteImage_NoUserIDInContext_ReturnsError() {
	removed, err := s.repo.DeleteImage(s.T().Context(), "b1", "img1")

	s.Require().Error(err)
	s.Nil(removed)
}
