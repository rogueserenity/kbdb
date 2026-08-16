package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type NewVerifierDiscoveryFallbackSuite struct {
	suite.Suite
}

func TestNewVerifierDiscoveryFallbackSuite(t *testing.T) {
	suite.Run(t, new(NewVerifierDiscoveryFallbackSuite))
}

// discoverylessIssuer starts an httptest.Server that serves ONLY
// /oauth2/jwks (an empty but valid JWKS document) and 404s everything
// else, including /.well-known/openid-configuration - simulating
// @workos/emulate's actual behavior (confirmed live against a real
// running container during this task's design).
func discoverylessIssuer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/jwks", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []any{}})
	})
	return httptest.NewServer(mux)
}

func (s *NewVerifierDiscoveryFallbackSuite) TestFallsBackToJWKSWhenDiscoveryUnavailable() {
	srv := discoverylessIssuer(s.T())
	defer srv.Close()

	verifier, err := NewVerifier(s.T().Context(), srv.URL, "test-audience")

	s.Require().NoError(err, "NewVerifier should fall back to JWKS-only construction, not fail, when discovery 404s")
	s.NotNil(verifier)
}

func (s *NewVerifierDiscoveryFallbackSuite) TestStillFailsForAGenuinelyUnreachableIssuer() {
	// A URL with nothing listening at all - neither discovery nor JWKS
	// available. This must still be a real error, not silently succeed
	// with a broken verifier.
	_, err := NewVerifier(s.T().Context(), "http://127.0.0.1:1", "test-audience")
	s.Require().Error(err)
}
