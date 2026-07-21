package dynamo

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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
				{"PK": &types.AttributeValueMemberS{Value: "vendor"}},
				{"PK": &types.AttributeValueMemberS{Value: "keyboard_size"}},
			},
		}, nil)

	categories, err := s.repo.ListCategories(context.Background())

	s.Require().NoError(err)
	s.Equal([]string{"keyboard_size", "vendor"}, categories)
}

func (s *LookupRepositorySuite) TestListCategories_Paginates() {
	lastKey := map[string]types.AttributeValue{"PK": &types.AttributeValueMemberS{Value: "keyboard_size"}}

	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.MatchedBy(func(in *dynamodb.ScanInput) bool {
			return len(in.ExclusiveStartKey) == 0
		})).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{"PK": &types.AttributeValueMemberS{Value: "vendor"}},
			},
			LastEvaluatedKey: lastKey,
		}, nil).Once()

	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.MatchedBy(func(in *dynamodb.ScanInput) bool {
			return len(in.ExclusiveStartKey) > 0
		})).
		Return(&dynamodb.ScanOutput{
			Items: []map[string]types.AttributeValue{
				{"PK": &types.AttributeValueMemberS{Value: "keyboard_size"}},
			},
		}, nil).Once()

	categories, err := s.repo.ListCategories(context.Background())

	s.Require().NoError(err)
	s.Equal([]string{"keyboard_size", "vendor"}, categories)
}

func (s *LookupRepositorySuite) TestListCategories_Empty_ReturnsEmptySliceNotNil() {
	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.Anything).
		Return(&dynamodb.ScanOutput{Items: []map[string]types.AttributeValue{}}, nil)

	categories, err := s.repo.ListCategories(context.Background())

	s.Require().NoError(err)
	s.NotNil(categories)
	s.Empty(categories)
}

func (s *LookupRepositorySuite) TestListCategories_ScanError_Propagates() {
	s.mockClient.EXPECT().
		Scan(mock.Anything, mock.Anything).
		Return(nil, errors.New("dynamodb: throttled"))

	categories, err := s.repo.ListCategories(context.Background())

	s.Require().Error(err)
	s.Nil(categories)
}
