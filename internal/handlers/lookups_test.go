package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/problem"
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

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups", nil)
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

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups", nil)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
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

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/vendor", nil)
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

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/vendor", nil)
	req.SetPathValue("category", "vendor")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetLookupSuite) TestGetCategory_KeyboardLayout_ReturnsTypedValues() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{
			Category: "keyboard_layout",
			Values:   []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}},
		}, nil)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/keyboard_layout", nil)
	req.SetPathValue("category", "keyboard_layout")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got struct {
		Values []repository.LayoutValue `json:"values"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal([]repository.LayoutValue{{Name: "WK", Sizes: []string{"60%", "65%"}}}, got.Values)
}

func (s *GetLookupSuite) TestGetCategory_KeyboardLayout_CorruptStoredShape_Returns500() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_layout").
		Return(&repository.Lookup{Category: "keyboard_layout", Values: []any{"WK"}}, nil)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/keyboard_layout", nil)
	req.SetPathValue("category", "keyboard_layout")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *GetLookupSuite) TestGetCategory_NotFound_Returns404() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "vendor").
		Return(nil, repository.ErrNotFound)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/vendor", nil)
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
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/vendor", strings.NewReader(body))
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
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/%20", strings.NewReader(`{"values":["a"]}`))
	req.SetPathValue("category", " ")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_EmptyCategory_Returns400() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/", strings.NewReader(`{"values":["a"]}`))
	req.SetPathValue("category", "")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_NonStringValue_Returns400() {
	req := s.newRequest(`{"values":["a", {"name":"b"}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) newKeyboardLayoutRequest(body string) *http.Request {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/keyboard_layout", strings.NewReader(body))
	req.SetPathValue("category", "keyboard_layout")
	return req
}

func (s *CreateLookupSuite) TestCreateCategory_KeyboardLayout_WrongShape_Returns400() {
	req := s.newKeyboardLayoutRequest(`{"values":["WK"]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_KeyboardLayout_MissingName_Returns400() {
	req := s.newKeyboardLayoutRequest(`{"values":[{"sizes":["60%"]}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_KeyboardLayout_SizeNotApproved_Returns400() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"60%", "65%"}}, nil)

	req := s.newKeyboardLayoutRequest(`{"values":[{"name":"WK","sizes":["60%","85%"]}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)

	var got struct {
		InvalidParams []problem.InvalidParam `json:"invalid_params"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Require().Len(got.InvalidParams, 1)
	s.Equal("values[WK].sizes", got.InvalidParams[0].Name)
	s.Contains(got.InvalidParams[0].Reason, "85%")
}

func (s *CreateLookupSuite) TestCreateCategory_KeyboardLayout_KeyboardSizeMissing_AllSizesInvalid() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(nil, repository.ErrNotFound)

	req := s.newKeyboardLayoutRequest(`{"values":[{"name":"WK","sizes":["60%"]}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
}

func (s *CreateLookupSuite) TestCreateCategory_KeyboardLayout_Succeeds() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"60%", "65%"}}, nil)
	values := []any{map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}}}
	s.mockRepo.EXPECT().
		CreateCategory(mock.Anything, "keyboard_layout", values).
		Return(&repository.Lookup{Category: "keyboard_layout", Values: values}, nil)

	req := s.newKeyboardLayoutRequest(`{"values":[{"name":"WK","sizes":["60%","65%"]}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
}

func (s *CreateLookupSuite) TestCreateCategory_BuildCaseMountType_WrongShape_Returns400() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/build_case_mount_type", strings.NewReader(`{"values":["Top Mount"]}`))
	req.SetPathValue("category", "build_case_mount_type")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *CreateLookupSuite) TestCreateCategory_BuildCaseMountType_Succeeds() {
	values := []any{map[string]any{"name": "Top Mount", "supports_durometer": false}}
	s.mockRepo.EXPECT().
		CreateCategory(mock.Anything, "build_case_mount_type", values).
		Return(&repository.Lookup{Category: "build_case_mount_type", Values: values}, nil)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/build_case_mount_type",
		strings.NewReader(`{"values":[{"name":"Top Mount","supports_durometer":false}]}`))
	req.SetPathValue("category", "build_case_mount_type")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusCreated, rec.Code)
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
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPut, "/v1/lookups/vendor", strings.NewReader(body))
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

func (s *ReplaceLookupSuite) TestReplaceCategory_NonStringValue_Returns400() {
	req := s.newRequest(`{"values":["a", {"name":"b"}]}`)
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *ReplaceLookupSuite) TestReplaceCategory_KeyboardLayout_SizeNotApproved_Returns400() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, "keyboard_size").
		Return(&repository.Lookup{Category: "keyboard_size", Values: []any{"60%"}}, nil)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPut, "/v1/lookups/keyboard_layout",
		strings.NewReader(`{"values":[{"name":"WK","sizes":["85%"]}]}`))
	req.SetPathValue("category", "keyboard_layout")
	rec := httptest.NewRecorder()

	s.handler(rec, req)

	s.Equal(http.StatusBadRequest, rec.Code)
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
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPut, "/v1/lookups/placeholder", strings.NewReader(`{"values":["a"]}`))
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
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodDelete, "/v1/lookups/placeholder", nil)
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
