package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type AuthorizerSubjectSuite struct {
	suite.Suite
}

func TestAuthorizerSubjectSuite(t *testing.T) {
	suite.Run(t, new(AuthorizerSubjectSuite))
}

func (s *AuthorizerSubjectSuite) TestNoHeaderReturnsFalse() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/", nil)
	sub, ok := authorizerSubject(req)
	s.False(ok)
	s.Empty(sub)
}

func (s *AuthorizerSubjectSuite) TestValidClaimsReturnsSubject() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Amzn-Request-Context",
		`{"authorizer":{"jwt":{"claims":{"sub":"user_01ABC","aud":"client_xyz"}}}}`)
	sub, ok := authorizerSubject(req)
	s.True(ok)
	s.Equal("user_01ABC", sub)
}

func (s *AuthorizerSubjectSuite) TestMalformedJSONReturnsFalse() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Amzn-Request-Context", `{not valid json`)
	sub, ok := authorizerSubject(req)
	s.False(ok)
	s.Empty(sub)
}

func (s *AuthorizerSubjectSuite) TestMissingSubClaimReturnsFalse() {
	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/", nil)
	req.Header.Set("X-Amzn-Request-Context",
		`{"authorizer":{"jwt":{"claims":{"aud":"client_xyz"}}}}`)
	sub, ok := authorizerSubject(req)
	s.False(ok)
	s.Empty(sub)
}
