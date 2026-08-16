package auth

import (
	"context"
	"fmt"
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

// NewVerifier constructs a Verifier for issuerURL via OIDC discovery.
// audience is the expected token audience (WorkOS's User Management
// application client_id in production).
func NewVerifier(ctx context.Context, issuerURL, audience string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery failed: %w", err)
	}

	return &Verifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
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
