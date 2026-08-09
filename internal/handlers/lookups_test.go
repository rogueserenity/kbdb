package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type ListLookupsSuite struct {
	suite.Suite
}

func TestListLookupsSuite(t *testing.T) {
	suite.Run(t, new(ListLookupsSuite))
}

func (s *ListLookupsSuite) TestListCategories_Succeeds() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups", nil)
	rec := httptest.NewRecorder()

	ListLookups(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got []string
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Contains(got, "vendor")
	s.Contains(got, "switch_material")
}

type GetLookupSuite struct {
	suite.Suite
}

func TestGetLookupSuite(t *testing.T) {
	suite.Run(t, new(GetLookupSuite))
}

func (s *GetLookupSuite) newRequest(category string) *http.Request {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/lookups/"+category, nil)
	req.SetPathValue("category", category)
	return req
}

func (s *GetLookupSuite) TestGetCategory_Succeeds() {
	req := s.newRequest("vendor")
	rec := httptest.NewRecorder()

	GetLookup(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("application/json", rec.Header().Get("Content-Type"))

	var got struct {
		Category string `json:"category"`
		Values   []any  `json:"values"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.Equal("vendor", got.Category)
	s.Contains(got.Values, "Amazon")
}

func (s *GetLookupSuite) TestGetCategory_KeyboardLayout_ReturnsTypedValues() {
	req := s.newRequest("keyboard_layout")
	rec := httptest.NewRecorder()

	GetLookup(rec, req)

	s.Equal(http.StatusOK, rec.Code)

	var got struct {
		Values []struct {
			Name  string   `json:"name"`
			Sizes []string `json:"sizes"`
		} `json:"values"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &got))
	s.NotEmpty(got.Values)
	s.NotEmpty(got.Values[0].Name)
	s.NotEmpty(got.Values[0].Sizes)
}

func (s *GetLookupSuite) TestGetCategory_NotFound_Returns404() {
	req := s.newRequest("not-a-real-category")
	rec := httptest.NewRecorder()

	GetLookup(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
