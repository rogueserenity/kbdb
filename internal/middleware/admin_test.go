package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/auth"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
)

type RequireAdminSuite struct {
	suite.Suite

	nextCalled bool
	handler    http.Handler
}

func TestRequireAdminSuite(t *testing.T) {
	suite.Run(t, new(RequireAdminSuite))
}

func (s *RequireAdminSuite) SetupTest() {
	s.nextCalled = false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.nextCalled = true
		w.WriteHeader(http.StatusOK)
	})
	s.handler = RequireAdmin(next)
}

func (s *RequireAdminSuite) TestWithAdminsGroup_CallsNext() {
	ctx := ctxpkg.WithGroups(s.T().Context(), []string{auth.AdminsGroup})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/lookups/vendor", nil)
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)

	s.True(s.nextCalled)
	s.Equal(http.StatusOK, rec.Code)
}

func (s *RequireAdminSuite) TestWithoutAdminsGroup_Returns403() {
	ctx := ctxpkg.WithGroups(s.T().Context(), []string{"engineering"})
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/v1/lookups/vendor", nil)
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)

	s.False(s.nextCalled)
	s.Equal(http.StatusForbidden, rec.Code)
	s.Equal("application/problem+json", rec.Header().Get("Content-Type"))
}

func (s *RequireAdminSuite) TestNoGroups_Returns403() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodPost, "/v1/lookups/vendor", nil)
	rec := httptest.NewRecorder()

	s.handler.ServeHTTP(rec, req)

	s.False(s.nextCalled)
	s.Equal(http.StatusForbidden, rec.Code)
}
