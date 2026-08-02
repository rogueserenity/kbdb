package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/stretchr/testify/suite"
)

const testSpec = `
openapi: "3.0.0"
info:
  version: 1.0.0
  title: TestSpec
paths:
  /widgets:
    post:
      operationId: createWidget
      responses:
        '204':
          description: No content
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                  minLength: 3
                count:
                  type: integer
                  minimum: 1
                  maximum: 10
              additionalProperties: false
  /items:
    get:
      operationId: listItems
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 100
      responses:
        '200':
          description: OK
`

type ValidationErrorHandlerSuite struct {
	suite.Suite

	server http.Handler
}

func TestValidationErrorHandlerSuite(t *testing.T) {
	suite.Run(t, new(ValidationErrorHandlerSuite))
}

func (s *ValidationErrorHandlerSuite) SetupTest() {
	spec, err := openapi3.NewLoader().LoadFromData([]byte(testSpec))
	s.Require().NoError(err)
	spec.Servers = nil

	mux := http.NewServeMux()
	mux.Handle("POST /widgets", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	mux.Handle("GET /items", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	validator := nethttpmiddleware.OapiRequestValidatorWithOptions(spec, &nethttpmiddleware.Options{
		Options: openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			MultiError:         true,
		},
		ErrorHandlerWithOpts: validationErrorHandler,
	})

	s.server = validator(mux)
}

func (s *ValidationErrorHandlerSuite) post(body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/widgets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	s.server.ServeHTTP(rec, req)

	return rec
}

func (s *ValidationErrorHandlerSuite) getItems(query string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/items?"+query, nil)

	rec := httptest.NewRecorder()
	s.server.ServeHTTP(rec, req)

	return rec
}

type invalidParamsBody struct {
	Detail        string `json:"detail"`
	InvalidParams []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"invalid_params"`
}

func (s *ValidationErrorHandlerSuite) TestValidField_Succeeds() {
	rec := s.post(`{"name":"abc"}`)

	s.Equal(http.StatusNoContent, rec.Code)
}

func (s *ValidationErrorHandlerSuite) TestSingleFieldViolation_NamesTheField() {
	rec := s.post(`{"name":"ab"}`)

	s.Equal(http.StatusBadRequest, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body invalidParamsBody
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("name", body.InvalidParams[0].Name)
}

// kin-openapi's object-schema validation stops at the first invalid
// property it walks within a single JSON object, even with
// openapi3filter.Options.MultiError set - name and count are both invalid
// here, but only one is ever reported. MultiError aggregates across
// separate validation steps (e.g. a param plus a body), not every property
// inside one object schema.
func (s *ValidationErrorHandlerSuite) TestMultipleFieldViolationsInOneObject_ReportsOnlyOne() {
	rec := s.post(`{"name":"ab","count":11}`)

	s.Equal(http.StatusBadRequest, rec.Code)

	var body invalidParamsBody
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
}

func (s *ValidationErrorHandlerSuite) TestMissingRequiredField_NamesTheField() {
	rec := s.post(`{}`)

	s.Equal(http.StatusBadRequest, rec.Code)

	var body invalidParamsBody
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("name", body.InvalidParams[0].Name)
}

func (s *ValidationErrorHandlerSuite) TestNonNumericQueryParam_NamesTheParam() {
	rec := s.getItems("limit=notanumber")

	s.Equal(http.StatusBadRequest, rec.Code)

	var body invalidParamsBody
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("limit", body.InvalidParams[0].Name)
}

func (s *ValidationErrorHandlerSuite) TestOutOfRangeQueryParam_NamesTheParam() {
	rec := s.getItems("limit=999")

	s.Equal(http.StatusBadRequest, rec.Code)

	var body invalidParamsBody
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.Require().Len(body.InvalidParams, 1)
	s.Equal("limit", body.InvalidParams[0].Name)
}

func (s *ValidationErrorHandlerSuite) TestUnmatchedRoute_Returns404() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/does-not-exist", nil)
	rec := httptest.NewRecorder()
	s.server.ServeHTTP(rec, req)

	s.Equal(http.StatusNotFound, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}
