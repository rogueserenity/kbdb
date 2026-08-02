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

type LookupRepositorySuite struct {
	suite.Suite

	mockClient *mocks.MockDynamoAPI
	repo       *LookupRepository
}

func TestLookupRepositorySuite(t *testing.T) {
	suite.Run(t, new(LookupRepositorySuite))
}

func (s *LookupRepositorySuite) SetupTest() {
	s.mockClient = mocks.NewMockDynamoAPI(s.T())
	s.repo = &LookupRepository{client: s.mockClient, tableName: "lookup-table"}
}

func (s *LookupRepositorySuite) TestListCategories_Succeeds() {
	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.Anything).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{"category": &types.AttributeValueMemberS{Value: "vendor"}},
				{"category": &types.AttributeValueMemberS{Value: "keyboard_size"}},
			},
		}, nil)

	categories, err := s.repo.ListCategories(s.T().Context())

	s.Require().NoError(err)
	s.Equal([]string{"keyboard_size", "vendor"}, categories)
}

func (s *LookupRepositorySuite) TestListCategories_Paginates() {
	lastKey := map[string]types.AttributeValue{"category": &types.AttributeValueMemberS{Value: "keyboard_size"}}

	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.MatchedBy(func(in *dynamodb.ScanInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{"category": &types.AttributeValueMemberS{Value: "vendor"}},
			},
			LastEvaluatedKey: lastKey,
		}, nil).Once()

	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.MatchedBy(func(in *dynamodb.ScanInput) bool {
			return len(in.ExclusiveStartKey) > 0
		})).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{"category": &types.AttributeValueMemberS{Value: "keyboard_size"}},
			},
		}, nil).Once()

	categories, err := s.repo.ListCategories(s.T().Context())

	s.Require().NoError(err)
	s.Equal([]string{"keyboard_size", "vendor"}, categories)
}

func (s *LookupRepositorySuite) TestListCategories_Empty_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.Anything).
		Return(&dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil)

	categories, err := s.repo.ListCategories(s.T().Context())

	s.Require().NoError(err)
	s.NotNil(categories)
	s.Empty(categories)
}

func (s *LookupRepositorySuite) TestListCategories_ScanError_Propagates() {
	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	categories, err := s.repo.ListCategories(s.T().Context())

	s.Require().Error(err)
	s.Nil(categories)
}

func (s *LookupRepositorySuite) TestGetCategory_Succeeds() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{
			Item: map[string]types.AttributeValue{
				"category": &types.AttributeValueMemberS{Value: "vendor"},
				"values": &types.AttributeValueMemberL{Value: []types.AttributeValue{
					&types.AttributeValueMemberS{Value: "a"},
					&types.AttributeValueMemberS{Value: "b"},
				}},
			},
		}, nil)

	lookup, err := s.repo.GetCategory(s.T().Context(), "vendor")

	s.Require().NoError(err)
	s.Equal(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, lookup)
}

func (s *LookupRepositorySuite) TestGetCategory_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(&dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{}}, nil)

	lookup, err := s.repo.GetCategory(s.T().Context(), "missing")

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestGetCategory_GetItemError_Propagates() {
	s.mockClient.EXPECT().
		GetItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	lookup, err := s.repo.GetCategory(s.T().Context(), "vendor")

	s.Require().Error(err)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestCreateCategory_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	lookup, err := s.repo.CreateCategory(s.T().Context(), "vendor", []any{"a", "b"})

	s.Require().NoError(err)
	s.Equal(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, lookup)
}

func (s *LookupRepositorySuite) TestCreateCategory_AlreadyExists_ReturnsErrAlreadyExists() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	lookup, err := s.repo.CreateCategory(s.T().Context(), "vendor", []any{"a"})

	s.Require().ErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestCreateCategory_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	lookup, err := s.repo.CreateCategory(s.T().Context(), "vendor", []any{"a"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrAlreadyExists)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestReplaceCategory_Succeeds() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(&dynamodb.PutItemOutput{}, nil)

	lookup, err := s.repo.ReplaceCategory(s.T().Context(), "vendor", []any{"c", "d"})

	s.Require().NoError(err)
	s.Equal(&repository.Lookup{Category: "vendor", Values: []any{"c", "d"}}, lookup)
}

func (s *LookupRepositorySuite) TestReplaceCategory_NotFound_ReturnsErrNotFound() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, &types.ConditionalCheckFailedException{})

	lookup, err := s.repo.ReplaceCategory(s.T().Context(), "vendor", []any{"a"})

	s.Require().ErrorIs(err, repository.ErrNotFound)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestReplaceCategory_PutItemError_Propagates() {
	s.mockClient.EXPECT().
		PutItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	lookup, err := s.repo.ReplaceCategory(s.T().Context(), "vendor", []any{"a"})

	s.Require().Error(err)
	s.Require().NotErrorIs(err, repository.ErrNotFound)
	s.Nil(lookup)
}

func (s *LookupRepositorySuite) TestDeleteCategory_Succeeds() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	err := s.repo.DeleteCategory(s.T().Context(), "vendor")

	s.Require().NoError(err)
}

func (s *LookupRepositorySuite) TestDeleteCategory_NonExistentCategory_StillSucceeds() {
	// Real DynamoDB DeleteItem returns no error for a missing key - this
	// confirms DeleteCategory doesn't invent a not-found translation.
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(&dynamodb.DeleteItemOutput{}, nil)

	err := s.repo.DeleteCategory(s.T().Context(), "does-not-exist")

	s.Require().NoError(err)
}

func (s *LookupRepositorySuite) TestDeleteCategory_DeleteItemError_Propagates() {
	s.mockClient.EXPECT().
		DeleteItem(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	err := s.repo.DeleteCategory(s.T().Context(), "vendor")

	s.Require().Error(err)
}
