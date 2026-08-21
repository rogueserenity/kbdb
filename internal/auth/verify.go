package auth

import (
	"context"
	"fmt"
	"slices"
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

// verifiedToken is the subset of *oidc.IDToken's behavior VerifyToken needs:
// the claims go-oidc always parses (Subject, Expiry, Audience). A real
// *oidc.IDToken satisfies this directly.
type verifiedToken interface {
	audience() []string
	subject() string
	expiry() time.Time
}

// oidcToken adapts *oidc.IDToken to verifiedToken.
type oidcToken struct{ *oidc.IDToken }

func (t oidcToken) audience() []string { return t.Audience }
func (t oidcToken) subject() string    { return t.Subject }
func (t oidcToken) expiry() time.Time  { return t.Expiry }

// tokenVerifier is the single method of *oidc.IDTokenVerifier that Verifier
// depends on. Depending on this interface rather than the concrete type lets
// tests inject a mock instead of standing up a real OIDC discovery/key set.
type tokenVerifier interface {
	Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error)
}

// Verifier verifies raw JWTs issued by a single OIDC provider.
type Verifier struct {
	verifier tokenVerifier
	audience string
}

// NewVerifier constructs a Verifier for issuerURL via OIDC discovery.
// audience is the expected token audience (the Stytch project ID in
// production - see checkAudience).
//
// The audience check itself is done by VerifyToken, not go-oidc's own
// Verify: go-oidc's built-in check (oidc.Config.ClientID) expects aud to
// match a single value exactly, but Stytch's tokens carry aud as a
// multi-value array (e.g. [project_id, ...]) that must merely contain the
// expected value - see checkAudience's slices.Contains. SkipClientIDCheck
// is set here so go-oidc's own (incompatible) check is bypassed in favor
// of that.
func NewVerifier(ctx context.Context, issuerURL, audience string) (*Verifier, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: OIDC discovery failed: %w", err)
	}

	return &Verifier{
		verifier: provider.Verifier(&oidc.Config{SkipClientIDCheck: true}),
		audience: audience,
	}, nil
}

// NewVerifierForTesting constructs a Verifier from an already-configured
// tokenVerifier, for tests in other packages that need to inject a mock
// (internal/auth/mocks.MockTokenVerifier) without a real OIDC discovery
// round-trip. Not used by production code - see NewVerifier for that.
func NewVerifierForTesting(v tokenVerifier, audience string) *Verifier {
	return &Verifier{verifier: v, audience: audience}
}

// VerifyToken verifies rawToken's signature, expiry, and issuer via the
// underlying tokenVerifier, then separately checks that its aud claim
// contains v.audience (see checkAudience). Returns the verified claims, or
// an error if the token or its audience is invalid.
func (v *Verifier) VerifyToken(ctx context.Context, rawToken string) (*Claims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, fmt.Errorf("auth: verifying token: %w", err)
	}

	token := oidcToken{idToken}
	if err := v.checkAudience(token); err != nil {
		return nil, fmt.Errorf("auth: verifying token: %w", err)
	}

	return &Claims{Subject: token.subject(), Expiry: token.expiry()}, nil
}

// checkAudience rejects token unless its aud claim contains v.audience.
// aud is checked as a multi-value array rather than requiring an exact
// match, since Stytch's tokens carry other values alongside the expected
// one (e.g. aud: [project_id, ...]) - see NewVerifier.
func (v *Verifier) checkAudience(token verifiedToken) error {
	aud := token.audience()
	if !slices.Contains(aud, v.audience) {
		return fmt.Errorf("expected audience %q, got %v", v.audience, aud)
	}

	return nil
}
