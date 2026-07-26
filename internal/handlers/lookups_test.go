package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

type CreateLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
	handler  http.HandlerFunc
}

func TestCreateLookupSuite(t *testing.T) {
	suite.Run(t, new(CreateLookupSuite))
}

func (s *CreateLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = CreateLookup(s.mockRepo)
}

func (s *CreateLookupSuite) newRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/lookups/vendor", strings.NewReader(body))
	req.SetPathValue("category", "vendor")
	return req
}

func (s *CreateLookupSuite) TestCreateCategory_Succeeds() {
	s.mockRepo.EXPECT().
		CreateCategory(mock.Anything, "vendor", []any{"a", "b"}).
		Return(&repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, nil)

	req := s.newRequest(`{"values":["a","b"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Lookup
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(repository.Lookup{Category: "vendor", Values: []any{"a", "b"}}, got)
}

func (s *CreateLookupSuite) TestCreateCategory_AlreadyExists_Returns409() {
	s.mockRepo.EXPECT().
		CreateCategory(mock.Anything, "vendor", []any{"a"}).
		Return(nil, repository.ErrAlreadyExists)

	req := s.newRequest(`{"values":["a"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusConflict, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		CreateCategory(mock.Anything, "vendor", []any{"a"}).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(`{"values":["a"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_InvalidBody_Returns400() {
	req := s.newRequest("not json")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_WhitespaceCategory_Returns400() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/lookups/%20", strings.NewReader(`{"values":["a"]}`))
	req.SetPathValue("category", " ")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_EmptyCategory_Returns400() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/lookups/", strings.NewReader(`{"values":["a"]}`))
	req.SetPathValue("category", "")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type ReplaceLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
	handler  http.HandlerFunc
}

func TestReplaceLookupSuite(t *testing.T) {
	suite.Run(t, new(ReplaceLookupSuite))
}

func (s *ReplaceLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = ReplaceLookup(s.mockRepo)
}

func (s *ReplaceLookupSuite) newRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/lookups/vendor", strings.NewReader(body))
	req.SetPathValue("category", "vendor")
	return req
}

func (s *ReplaceLookupSuite) TestReplaceCategory_Succeeds() {
	s.mockRepo.EXPECT().
		ReplaceCategory(mock.Anything, "vendor", []any{"c", "d"}).
		Return(&repository.Lookup{Category: "vendor", Values: []any{"c", "d"}}, nil)

	req := s.newRequest(`{"values":["c","d"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got repository.Lookup
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal(repository.Lookup{Category: "vendor", Values: []any{"c", "d"}}, got)
}

func (s *ReplaceLookupSuite) TestReplaceCategory_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		ReplaceCategory(mock.Anything, "vendor", []any{"a"}).
		Return(nil, repository.ErrNotFound)

	req := s.newRequest(`{"values":["a"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ReplaceLookupSuite) TestReplaceCategory_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		ReplaceCategory(mock.Anything, "vendor", []any{"a"}).
		Return(nil, errors.New("put item failed"))

	req := s.newRequest(`{"values":["a"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ReplaceLookupSuite) TestReplaceCategory_InvalidBody_Returns400() {
	req := s.newRequest("not json")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ReplaceLookupSuite) TestReplaceCategory_BlankCategory_Returns400() {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/lookups/placeholder", strings.NewReader(`{"values":["a"]}`))
	req.SetPathValue("category", " ")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

type DeleteLookupSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
	handler  http.HandlerFunc
}

func TestDeleteLookupSuite(t *testing.T) {
	suite.Run(t, new(DeleteLookupSuite))
}

func (s *DeleteLookupSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
	s.handler = DeleteLookup(s.mockRepo)
}

func (s *DeleteLookupSuite) newRequest(category string) *http.Request {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/lookups/placeholder", nil)
	req.SetPathValue("category", category)
	return req
}

func (s *DeleteLookupSuite) TestDeleteCategory_Succeeds() {
	s.mockRepo.EXPECT().
		DeleteCategory(mock.Anything, "vendor").
		Return(nil)

	req := s.newRequest("vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
	s.Empty(rec.Body.Bytes())
}

func (s *DeleteLookupSuite) TestDeleteCategory_RepositoryError_Returns500() {
	s.mockRepo.EXPECT().
		DeleteCategory(mock.Anything, "vendor").
		Return(errors.New("delete item failed"))

	req := s.newRequest("vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *DeleteLookupSuite) TestDeleteCategory_BlankCategory_Returns400() {
	req := s.newRequest(" ")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
