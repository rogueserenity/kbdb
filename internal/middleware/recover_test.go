package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type RecoverSuite struct {
	suite.Suite
}

func TestRecoverSuite(t *testing.T) {
	suite.Run(t, new(RecoverSuite))
}

func (s *RecoverSuite) TestNextPanics_Returns500ProblemJSON() {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	handler := Recover(next)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/switches", nil)
	rec := httptest.NewRecorder()

	s.NotPanics(func() {
		handler.ServeHTTP(rec, req)
	})

	s.Equal(http.StatusInternalServerError, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))

	var body struct {
		Title string `json:"title"`
	}
	s.Require().NoError(json.Unmarshal(rec.Body.Bytes(), &body))
	s.NotEmpty(body.Title)
}

func (s *RecoverSuite) TestNextSucceeds_PassesThrough() {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := Recover(next)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/switches", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
}

func (s *RecoverSuite) TestNextPanicsAfterWritingHeader_DoesNotWriteASecondBody() {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"partial":`))
		panic("boom")
	})
	handler := Recover(next)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/switches", nil)
	underlying := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: underlying, status: http.StatusOK}

	s.NotPanics(func() {
		handler.ServeHTTP(rec, req)
	})

	s.Equal(http.StatusOK, underlying.Code)
	s.Equal(`{"partial":`, underlying.Body.String())
}

func (s *RecoverSuite) TestNextPanicsWithErrAbortHandler_RePanics() {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})
	handler := Recover(next)

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/switches", nil)
	rec := httptest.NewRecorder()

	s.PanicsWithValue(http.ErrAbortHandler, func() {
		handler.ServeHTTP(rec, req)
	})
}
