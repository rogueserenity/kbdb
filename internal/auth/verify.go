package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Claims holds the subset of verified token claims the rest of the
// application cares about. A struct (rather than a bare string) so more
// fields can be added later without changing VerifyToken's signature.
type Claims struct {
	Subject string
	// Expiry is the token's exp claim.
	Expiry time.Time
}

// tokenVerifier is the single method of *oidc.IDTokenVerifier that Verifier
// depends on. Depending on this interface rather than the concrete type lets
// tests inject a mock instead of standing up a real OIDC discovery/key set.
type tokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// Verifier verifies raw JWTs issued by a single OIDC provider.
type Verifier struct {
	verifier tokenVerifier
}

// NewVerifier constructs a Verifier for issuerURL. audience is the
// expected token audience (WorkOS's User Management application
// client_id in production).
//
// Tries OIDC discovery first; falls back to a plain JWKS fetch if that
// fails, since @workos/emulate (the local dev/test stand-in for WorkOS)
// doesn't serve a discovery document.
func NewVerifier(ctx context.Context, issuerURL, audience string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err == nil {
		return &Verifier{
			verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
		}, nil
	}

	jwksURL := issuerURL + "/oauth2/jwks"

	// oidc.NewRemoteKeySet never itself returns an error - it fetches
	// lazily on first Verify() call, and a fake/garbage token would fail
	// to parse regardless of whether the JWKS endpoint is even
	// reachable, so probing with one wouldn't actually prove reachability.
	// Instead, GET the JWKS URL directly here so an issuer that supports
	// neither discovery nor JWKS still fails NewVerifier itself (matching
	// its existing contract: functions/api/main.go treats a NewVerifier
	// error as fatal at startup, not something to retry per-request).
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if reqErr != nil {
		return nil, fmt.Errorf("auth: building JWKS fallback request: %w", reqErr)
	}
	resp, getErr := http.DefaultClient.Do(req)
	if getErr != nil {
		return nil, fmt.Errorf("auth: OIDC discovery failed (%w) and JWKS fallback unreachable: %w", err, getErr)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: OIDC discovery failed (%w) and JWKS fallback returned %s", err, resp.Status)
	}

	keySet := oidc.NewRemoteKeySet(ctx, jwksURL)
	return &Verifier{
		verifier: oidc.NewVerifier(issuerURL, keySet, &oidc.Config{ClientID: audience}),
	}, nil
}

// NewVerifierForTesting constructs a Verifier from an already-configured
// tokenVerifier, for tests in other packages that need to inject a mock
// (internal/auth/mocks.MockTokenVerifier) without a real OIDC discovery
// round-trip. Not used by production code - see NewVerifier for that.
func NewVerifierForTesting(v tokenVerifier) *Verifier {
	return &Verifier{verifier: v}
}

// VerifyToken verifies rawToken's signature, expiry, issuer, and audience,
// returning the verified claims or an error if the token is invalid.
func (v *Verifier) VerifyToken(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("auth: verifying token: %w", err)
	}

	return &Claims{Subject: idToken.Subject, Expiry: idToken.Expiry}, nil
}
