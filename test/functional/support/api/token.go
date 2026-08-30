package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/rogueserenity/oidc-testkit/pkg/oidctest"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var (
	signerOnce sync.Once
	signer     *oidctest.Signer
	signerErr  error
)

func sharedSigner() (*oidctest.Signer, error) {
	signerOnce.Do(func() {
		path := support.OidcSigningKeyPath()
		if path == "" {
			signerErr = fmt.Errorf("KBDB_OIDC_SIGNING_KEY_PATH is not set - run scripts/func-setup.sh first")
			return
		}
		pemBytes, err := os.ReadFile(path) //nolint:gosec // path is a deploy-controlled env var
		if err != nil {
			signerErr = fmt.Errorf("reading signing key %s: %w", path, err)
			return
		}
		key, err := oidctest.LoadKey(pemBytes)
		if err != nil {
			signerErr = fmt.Errorf("parsing signing key %s: %w", path, err)
			return
		}
		signer = oidctest.NewSigner(key, support.OidcIssuer(), support.OidcAudience())
	})
	return signer, signerErr
}

// NewAuthIdentity mints a token for a fresh random subject and returns both,
// so a spec can seed and assert data owned by that subject.
func NewAuthIdentity(_ context.Context) (token, subject string, err error) {
	s, err := sharedSigner()
	if err != nil {
		return "", "", err
	}
	return s.Sign()
}

// fixtureIdentity is one (token, subject) pair minted once and reused, so
// repeated AuthToken/SecondUserAuthToken calls in a spec return the same
// identity - as the old fixed IdP fixture users did.
type fixtureIdentity struct {
	once    sync.Once
	token   string
	subject string
	err     error
}

func (f *fixtureIdentity) get() (string, error) {
	f.once.Do(func() {
		s, err := sharedSigner()
		if err != nil {
			f.err = err
			return
		}
		f.token, f.subject, f.err = s.Sign()
	})
	return f.token, f.err
}

var (
	firstFixtureIdentity  fixtureIdentity
	secondFixtureIdentity fixtureIdentity
)

// AuthToken returns a token for the primary test identity.
//
// Migration shim for specs not yet on NewAuthIdentity; removed with
// SecondUserAuthToken and TokenSubject once they are.
func AuthToken(_ context.Context) (string, error) {
	return firstFixtureIdentity.get()
}

// SecondUserAuthToken returns a token for a second test identity, distinct
// from AuthToken's. Migration shim - see AuthToken.
func SecondUserAuthToken(_ context.Context) (string, error) {
	return secondFixtureIdentity.get()
}

// TokenSubject returns a token's "sub" claim, unverified. Migration shim -
// NewAuthIdentity returns the subject directly.
func TokenSubject(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed token: expected 3 dot-separated parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decoding token payload: %w", err)
	}

	var claims struct {
		Subject string `json:"sub"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("decoding token claims: %w", err)
	}
	if claims.Subject == "" {
		return "", fmt.Errorf("token missing sub claim")
	}

	return claims.Subject, nil
}
