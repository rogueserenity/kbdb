// Package support holds env-var-derived configuration shared by every
// functional test suite (base URL, table names, mockoidc credentials) - the
// common ground support/api's HTTP clients and support/db's DynamoDB
// seed/cleanup helpers both build on, so neither has to import the other.
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

// MockOIDCBaseURL returns the mockoidc instance's base address (no /oidc
// suffix). Configurable via KBDB_MOCKOIDC_BASE_URL.
func MockOIDCBaseURL() string {
	if v := os.Getenv("KBDB_MOCKOIDC_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:9999"
}

// MockOIDCTokenURL returns the mockoidc instance's OIDC issuer base, used to
// mint test tokens. Not used when pointed at a real deployed stack (issue
// #8's real-Cognito-token path is separate).
func MockOIDCTokenURL() string {
	return MockOIDCBaseURL() + "/oidc"
}

// LookupTableName returns the DynamoDB table name backing GET /v1/lookups,
// so specs can seed fixture rows directly.
func LookupTableName() string {
	return os.Getenv("KBDB_LOOKUP_TABLE_NAME")
}

// SwitchTableName returns the DynamoDB table name backing
// GET /users/{userId}/switches, so specs can seed fixture rows directly.
func SwitchTableName() string {
	return os.Getenv("KBDB_SWITCH_TABLE_NAME")
}

// KeyboardTableName returns the DynamoDB table name backing
// GET /users/{userId}/keyboards, so specs can seed fixture rows directly.
func KeyboardTableName() string {
	return os.Getenv("KBDB_KEYBOARD_TABLE_NAME")
}

// KeycapSetTableName returns the DynamoDB table name backing
// GET /users/{userId}/keycap-sets, so specs can seed fixture rows directly.
func KeycapSetTableName() string {
	return os.Getenv("KBDB_KEYCAP_SET_TABLE_NAME")
}

// DynamoDBEndpointURL returns the DynamoDB endpoint specs should talk to.
// Empty uses default AWS endpoint resolution.
func DynamoDBEndpointURL() string {
	return os.Getenv("KBDB_DYNAMODB_ENDPOINT_URL")
}
