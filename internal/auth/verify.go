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
	// Groups is the token's cognito:groups claim. Empty if absent.
	Groups []string
	// Expiry is the token's exp claim.
	Expiry time.Time
}

// cognitoGroupsClaims reads cognito:groups via IDToken.Claims - it's
// Cognito-specific, not exposed as an oidc.IDToken field.
type cognitoGroupsClaims struct {
	Groups []string `json:"cognito:groups"`
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

// NewVerifier constructs a Verifier by fetching the OIDC discovery document
// at issuerURL. audience is the expected token audience (the Cognito User
// Pool Client ID in production).
func NewVerifier(ctx context.Context, issuerURL, audience string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: fetching OIDC provider metadata: %w", err)
	}

	return &Verifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: audience}),
	}, nil
}

// VerifyToken verifies rawToken's signature, expiry, issuer, and audience,
// returning the verified claims or an error if the token is invalid.
func (v *Verifier) VerifyToken(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("auth: verifying token: %w", err)
	}

	// Missing claim isn't an error - just leaves Groups nil.
	var groups cognitoGroupsClaims
	_ = idToken.Claims(&groups)

	return &Claims{Subject: idToken.Subject, Groups: groups.Groups, Expiry: idToken.Expiry}, nil
}
