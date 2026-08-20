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
// the claims go-oidc always parses (Subject, Expiry, Audience) plus a way to
// read the client_id claim it doesn't parse itself. A real *oidc.IDToken
// satisfies this directly - Claims unmarshals its raw JSON payload, which
// go-oidc only populates via a real Verify() call, so tests construct a
// fake implementation instead of a zero-value *oidc.IDToken (whose Claims
// would fail with "oidc: claims not set").
type verifiedToken interface {
	audience() []string
	subject() string
	expiry() time.Time
	clientID() (string, error)
}

// oidcToken adapts *oidc.IDToken to verifiedToken.
type oidcToken struct{ *oidc.IDToken }

func (t oidcToken) audience() []string { return t.Audience }
func (t oidcToken) subject() string    { return t.Subject }
func (t oidcToken) expiry() time.Time  { return t.Expiry }

func (t oidcToken) clientID() (string, error) {
	var claims struct {
		ClientID string `json:"client_id"`
	}
	if err := t.Claims(&claims); err != nil {
		return "", fmt.Errorf("reading client_id claim: %w", err)
	}
	return claims.ClientID, nil
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
	audience string
}

// NewVerifier constructs a Verifier for issuerURL via OIDC discovery.
// audience is the expected token audience (WorkOS's User Management
// application client_id in production).
//
// The audience check itself is done by VerifyToken, not go-oidc's own
// Verify - WorkOS access tokens obtained without an RFC 8707 resource
// parameter (e.g. authkit-js's plain SPA sign-in, as opposed to an MCP
// client's resource-scoped flow) carry no aud claim at all, only client_id.
// go-oidc's Verify only ever checks aud, so SkipClientIDCheck is set here
// and VerifyToken instead applies the same aud-or-client_id fallback rule
// API Gateway's native JWT authorizer already uses (see
// template.yaml's OidcAuthorizer): check aud if present, otherwise fall
// back to client_id.
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
// underlying tokenVerifier, then separately checks audience: if the token
// carries an aud claim, it must contain v.audience; otherwise (see
// NewVerifier) v.audience must match the token's client_id claim. Returns
// the verified claims, or an error if the token or its audience is invalid.
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

// checkAudience applies the aud-or-client_id rule described on NewVerifier.
func (v *Verifier) checkAudience(token verifiedToken) error {
	aud := token.audience()
	if slices.Contains(aud, v.audience) {
		return nil
	}
	if len(aud) > 0 {
		return fmt.Errorf("expected audience %q, got %v", v.audience, aud)
	}

	clientID, err := token.clientID()
	if err != nil {
		return err
	}
	if clientID != v.audience {
		return fmt.Errorf("expected client_id %q, got %q", v.audience, clientID)
	}

	return nil
}
