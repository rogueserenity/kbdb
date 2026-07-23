// Package support holds shared functional-test helpers: base URL
// configuration and mockoidc token minting. Used by every feature spec so
// the same client/assertions can point at a local sam local start-api
// instance now, or a real deployed stack later (issue #8), by changing only
// environment variables - no code changes.
package support

import (
	"context"
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

// mockOIDCBaseURL returns the mockoidc instance's base address (no /oidc
// suffix). Configurable via KBDB_MOCKOIDC_BASE_URL; MockOIDCTokenURL and
// QueueUser's control endpoint both derive from this one value, since
// they're the same server/port.
func mockOIDCBaseURL() string {
	if v := os.Getenv("KBDB_MOCKOIDC_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9999"
}

// MockOIDCTokenURL returns the mockoidc instance's OIDC issuer base, used to
// mint test tokens. Not used when pointed at a real deployed stack (issue
// #8's real-Cognito-token path is separate).
func MockOIDCTokenURL() string {
	return mockOIDCBaseURL() + "/oidc"
}

// authToken mints a token for the given fixture identity via mockoidc,
// unless envOverride is set (a real Cognito-minted token, when BaseURL
// points at a real deployed stack instead) - a local mockoidc instance and
// real Cognito aren't interchangeable token issuers, so this can't be
// derived from BaseURL alone.
func authToken(ctx context.Context, envOverride, subject string, groups []string) (string, error) {
	if v := os.Getenv(envOverride); v != "" {
		return v, nil
	}
	if err := QueueUser(ctx, mockOIDCBaseURL(), subject, groups); err != nil {
		return "", err
	}
	return MintToken(ctx, MockOIDCTokenURL(), MockOIDCClientID, MockOIDCClientSecret)
}

// AuthToken returns a valid bearer token for the plain (non-admin) test
// user. See authToken for the KBDB_AUTH_TOKEN override behavior.
func AuthToken(ctx context.Context) (string, error) {
	return authToken(ctx, "KBDB_AUTH_TOKEN", fixtures.TestUserSubject, nil)
}

// AdminAuthToken returns a valid bearer token for a user in the "admins"
// Cognito group (see template.yaml's AdminsGroup and
// internal/auth.Claims.Groups) - for exercising the write-gated lookup
// routes (PUT/POST/DELETE /v1/lookups/{category}). See authToken for the
// KBDB_ADMIN_AUTH_TOKEN override behavior.
func AdminAuthToken(ctx context.Context) (string, error) {
	return authToken(ctx, "KBDB_ADMIN_AUTH_TOKEN", fixtures.AdminUserSubject, fixtures.AdminGroups)
}

// LookupTableName returns the DynamoDB table name backing GET /v1/lookups,
// so specs can seed fixture rows directly.
func LookupTableName() string {
	return os.Getenv("KBDB_LOOKUP_TABLE_NAME")
}

// DynamoDBEndpointURL returns the DynamoDB endpoint specs should talk to.
// Empty uses default AWS endpoint resolution.
func DynamoDBEndpointURL() string {
	return os.Getenv("KBDB_DYNAMODB_ENDPOINT_URL")
}
