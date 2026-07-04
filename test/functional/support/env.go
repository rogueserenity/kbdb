// Package support holds shared functional-test helpers: base URL
// configuration and mockoidc token minting. Used by every feature spec so
// the same client/assertions can point at a local sam local start-api
// instance now, or a real deployed stack later (issue #8), by changing only
// environment variables - no code changes.
package support

import (
	"os"

	"github.com/rogueserenity/kbdb/test/functional/support/mockoidc/fixtures"
)

// MockOIDCClientID/MockOIDCClientSecret are the fixed test credentials
// mockoidc is configured with (see test/functional/support/mockoidc/main.go)
// - re-exported here so specs use one shared reference rather than
// hardcoding the same string twice.
const (
	MockOIDCClientID     = fixtures.TestClientID
	MockOIDCClientSecret = fixtures.TestClientSecret
)

// BaseURL returns the API's base URL under test, e.g. "http://127.0.0.1:3000"
// for a local sam local start-api instance or the real deployed kbdb-dev API
// Gateway URL. Configurable via KBDB_API_BASE_URL.
func BaseURL() string {
	if v := os.Getenv("KBDB_API_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:3000"
}

// MockOIDCTokenURL returns the mockoidc instance's OIDC issuer base, used to
// mint test tokens. Not used when pointed at a real deployed stack (issue
// #8's real-Cognito-token path is separate).
func MockOIDCTokenURL() string {
	if v := os.Getenv("KBDB_MOCKOIDC_ISSUER_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9999/oidc"
}

// AuthToken returns a valid bearer token for the API under test. By default
// it mints one from the local mockoidc instance. If KBDB_AUTH_TOKEN is set
// (e.g. a real Cognito-minted token, when BaseURL points at a real deployed
// stack instead of a local sam local start-api instance), that value is
// used instead - this is the one piece of "same client, different base
// URL" that can't be derived from BaseURL alone, since a local mockoidc
// instance and real Cognito aren't interchangeable token issuers.
func AuthToken() (string, error) {
	if v := os.Getenv("KBDB_AUTH_TOKEN"); v != "" {
		return v, nil
	}
	return MintToken(MockOIDCTokenURL(), MockOIDCClientID, MockOIDCClientSecret)
}
