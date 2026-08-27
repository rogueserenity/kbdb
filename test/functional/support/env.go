package support

import "os"

// EmulatorClientID/EmulatorClientSecret are the WorkOS emulator's seeded
// Connect application credentials (see scripts/workos-emulate-seed.yaml).
// sk_test_default is the emulator's fixed default API key/secret,
// documented by @workos/emulate itself - not a real secret.
const (
	EmulatorClientID     = "client_local_kbdb"
	EmulatorClientSecret = "sk_test_default"
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

// EmulatorBaseURL returns the WorkOS emulator's base address, used to mint
// test tokens locally. Not used when pointed at a real deployed stack (CI's
// real-issuer-token path is separate - see api.AuthToken).
// Configurable via KBDB_EMULATOR_BASE_URL.
func EmulatorBaseURL() string {
	if v := os.Getenv("KBDB_EMULATOR_BASE_URL"); v != "" {
		return v
	}
	return "http://127.0.0.1:4100"
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

// BuildTableName returns the DynamoDB table name backing
// POST /users/{userId}/builds, so specs can clean up fixture rows directly.
func BuildTableName() string {
	return os.Getenv("KBDB_BUILD_TABLE_NAME")
}

// ProfileTableName returns the DynamoDB table name backing
// GET /v1/profile/{identifier}, so specs can seed fixture rows directly.
func ProfileTableName() string {
	return os.Getenv("KBDB_PROFILE_TABLE_NAME")
}

// ProfileUsernameTableName returns the DynamoDB table name holding
// { username -> user_id } claim items, so specs can seed the claim needed
// to resolve a profile by username.
func ProfileUsernameTableName() string {
	return os.Getenv("KBDB_PROFILE_USERNAME_TABLE_NAME")
}

// DynamoDBEndpointURL returns the DynamoDB endpoint specs should talk to.
// Empty uses default AWS endpoint resolution.
func DynamoDBEndpointURL() string {
	return os.Getenv("KBDB_DYNAMODB_ENDPOINT_URL")
}
