package mcp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type HandleListLookupsSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestHandleListLookupsSuite(t *testing.T) {
	suite.Run(t, new(HandleListLookupsSuite))
}

func (s *HandleListLookupsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleListLookupsSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return([]string{"vendor", "switch_type"}, nil)

	handler := handleListLookups(s.mockRepo)
	result, out, err := handler(s.T().Context(), nil, schema.ListLookupsInput{})

	s.Require().NoError(err)
	s.Nil(result)
	s.Equal(schema.ListLookupsOutput{Categories: []string{"vendor", "switch_type"}}, out)
}

func (s *HandleListLookupsSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return(nil, errors.New("scan failed"))

	handler := handleListLookups(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.ListLookupsInput{})

	s.Require().Error(err)
}

type HandleGetLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestHandleGetLookupSuite(t *testing.T) {
	suite.Run(t, new(HandleGetLookupSuite))
}

func (s *HandleGetLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *HandleGetLookupSuite) TestSucceeds() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, nil)

	handler := handleGetLookup(s.mockRepo)
	result, out, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "vendor"})

	s.Require().NoError(err)
	s.Nil(result)
	s.Equal(schema.GetLookupOutput{Category: "vendor", Values: []any{"a", "b"}}, out)
}

func (s *HandleGetLookupSuite) TestNotFound_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "missing"})

	s.Require().Error(err)
}

func (s *HandleGetLookupSuite) TestRepositoryError_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, errors.New("get item failed"))

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: "vendor"})

	s.Require().Error(err)
}

func (s *HandleGetLookupSuite) TestKeyboardLayout_CorruptStoredShape_ReturnsError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeyboardLayout).
		Return(&repository.Lookup{Category: repository.CategoryKeyboardLayout, Values: []any{"WK"}}, nil)

	handler := handleGetLookup(s.mockRepo)
	_, _, err := handler(s.T().Context(), nil, schema.GetLookupInput{Category: repository.CategoryKeyboardLayout})

	s.Require().Error(err)
}
