// Package auth verifies OAuth2/OIDC JWTs against a discovery URL, independent
// of any particular issuer (Cognito in production, any OIDC-compliant mock
// issuer in tests) and independent of net/http, Lambda, or mcp-go.
package auth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Claims holds the subset of verified token claims the rest of the
// application cares about. A struct (rather than a bare string) so more
// fields can be added later without changing VerifyToken's signature.
type Claims struct {
	Subject string
	// Groups is the token's cognito:groups claim: the Cognito User Pool
	// Groups the subject belongs to (e.g. "admins"). Empty for tokens with
	// no group memberships, or for non-Cognito OIDC issuers that don't set
	// this claim.
	Groups []string
}

// cognitoGroupsClaims is the shape of the subset of an ID token's raw JSON
// payload this package reads beyond the fields go-oidc already exposes on
// oidc.IDToken (Subject, etc.). cognito:groups is Cognito-specific, not part
// of the OIDC core spec, so it has to be read via IDToken.Claims rather than
// a dedicated field.
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

	// cognito:groups is absent for tokens from non-Cognito issuers and for
	// Cognito users with no group memberships - not an error condition, so
	// a failure to unmarshal it (including simply not being present) just
	// leaves groups nil rather than rejecting an otherwise-valid token.
	var groups cognitoGroupsClaims
	_ = idToken.Claims(&groups)

	return &Claims{Subject: idToken.Subject, Groups: groups.Groups}, nil
}
