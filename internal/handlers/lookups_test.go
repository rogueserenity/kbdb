package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ListLookupsSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
	handler  http.HandlerFunc
}

func TestListLookupsSuite(t *testing.T) {
	suite.Run(t, new(ListLookupsSuite))
}

func (s *ListLookupsSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = ListLookups(s.mockRepo)
}

func (s *ListLookupsSuite) TestListCategories_Succeeds() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return([]string{"vendor", "switch_material"}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups", nil)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got []string
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal([]string{"vendor", "switch_material"}, got)
}

func (s *ListLookupsSuite) TestListCategories_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return(nil, errors.New("scan failed"))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups", nil)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ListLookupsSuite) TestListCategories_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		ListCategories(mock.Anything).
		Return(nil, repository.ErrNotFound)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups", nil)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type GetLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
	handler  http.HandlerFunc
}

func TestGetLookupSuite(t *testing.T) {
	suite.Run(t, new(GetLookupSuite))
}

func (s *GetLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = GetLookup(s.mockRepo)
}

func (s *GetLookupSuite) TestGetCategory_Succeeds() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, nil)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups/vendor", nil)
	req.SetPathValue("category", "vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Lookup
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, got)
}

func (s *GetLookupSuite) TestGetCategory_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, errors.New("get item failed"))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups/vendor", nil)
	req.SetPathValue("category", "vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetLookupSuite) TestGetCategory_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, repository.ErrNotFound)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/lookups/vendor", nil)
	req.SetPathValue("category", "vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
