# WorkOS Auth Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace kbdb's Cognito-based authentication with WorkOS (User
Management + first-party Connect), swap the API Gateway authorizer from
Cognito's JWT authorizer to a native WorkOS-configured JWT authorizer, and
overhaul local/CI test infrastructure to support it.

**Architecture:** All required-auth REST routes and all MCP routes drop
in-process token verification entirely and rely solely on API Gateway's
native `AuthorizerType: JWT` authorizer (config-only, no Lambda),
configured against WorkOS Connect's issuer/audience. Optional-auth REST
GET routes keep `Authorizer: NONE` and in-process verification via
`internal/auth`, reconfigured to WorkOS's issuer/JWKS instead of
Cognito's. Local dev replaces `mockoidc` with WorkOS's official Docker
emulator (`ghcr.io/workos/emulate`), run live on the same Docker network
as `sam local start-api`. CI cannot reach a live emulator from real
deployed API Gateway, so it publishes the emulator's (pinned-key,
therefore fully static) JWKS to S3 before `sam deploy`, and separately
runs the emulator locally within the CI job purely to mint test tokens.

**Tech Stack:** Go 1.26, AWS SAM/Lambda/API Gateway HttpApi, DynamoDB,
`go-oidc` v3, `@workos/emulate` (Docker), AWS S3, GitHub Actions, Ginkgo/testify.

**Spec:** `docs/superpowers/specs/2026-08-16-workos-auth-migration-design.md`
(supersedes `docs/superpowers/specs/2026-08-15-mcp-dcr-authorizer-design.md`,
retained for its research trail)

## Global Constraints

- Cognito resources (`UserPool`, `UserPoolClient`, `UserPoolDomain`) and
  their outputs are removed from `template.yaml` entirely, not left
  disabled/unused.
- `internal/auth`'s package boundary (`net/http`-unaware, per
  `CLAUDE.md`) does not change — only its issuer/audience configuration
  and its set of callers change.
- Required-auth REST routes and all three MCP routes stop calling
  `internal/auth`'s `VerifyToken` in-process (native authorizer only);
  optional-auth REST GET routes keep calling it (native authorizer can't
  do partial/anonymous-allowed auth — confirmed no AWS mechanism exists
  for this).
- `mockoidc` (`test/functional/support/mockoidc/`) is deleted entirely,
  including its `cognito:groups`-claim shim.
- Local dev's WorkOS emulator runs as a Docker container
  (`ghcr.io/workos/emulate`), on the same `docker-compose.yml` network as
  today's `mockoidc`/`localstack` services — not via `npx`/Node in the
  dev loop.
- CI's stack deploy must not depend on a dynamically-assigned URL
  (no tunneling) — the emulator's JWKS must be published to a stable
  location before `sam deploy` runs.
- Every task that touches `template.yaml`, `docker-compose.yml`, or
  `.github/workflows/ci.yml` must keep `mise run func-setup` /
  `mise run func-test` / a real CI run working at each task boundary —
  no task should leave functional tests unable to run at all.

---

## File Structure

**New:**
- `docs/superpowers/specs/2026-08-16-workos-auth-migration-design.md` — already exists (this plan implements it)
- `scripts/ci-publish-emulator-jwks.sh` — generates the pinned RSA keypair (or reads a checked-in one), computes static JWKS + OIDC discovery JSON, publishes both to S3
- `scripts/workos-emulate-seed.yaml` — seed config for the local Docker emulator (fixed Connect OAuth application `client_id`)
- `internal/auth/mocks/mock_TokenVerifier.go` — unchanged path, regenerated content (mockery-generated, not hand-edited)

**Modified:**
- `template.yaml` — authorizer swap, Cognito resource removal, env var updates, route `Auth` block removal for required-auth routes
- `internal/router/router.go` — remove `middleware.Auth(verifier)(...)` wrapping on required-auth routes (14 call sites)
- `internal/mcp/server.go` — remove `requireBearerToken`/`tokenVerifier` wrapping
- `internal/middleware/auth.go` — `Auth()` function removed; `OptionalAuth()` stays
- `functions/api/main.go` — still constructs `auth.Verifier` (needed by `OptionalAuth`), passes to `router.New`
- `functions/api/config.go` — `OIDC_ISSUER_URL`/`OIDC_AUDIENCE` now point at WorkOS
- `docker-compose.yml` — `mockoidc` service replaced with `workos-emulate` service
- `test/functional/support/mockoidc/` — deleted (directory and all contents)
- `test/functional/support/api/token.go` — `QueueUser`/`MintToken` reworked to call the emulator's real HTTP API
- `test/functional/support/env.go` — `MockOIDCBaseURL`/`MockOIDCTokenURL` replaced with emulator equivalents
- `test/functional/support/env.local.json` — `OIDC_ISSUER_URL`/`OIDC_AUDIENCE` point at the local emulator
- `scripts/func-setup.sh` — no functional change needed (docker-compose handles the swap), comment updates only
- `scripts/ci-create-test-user.sh` — replaced by emulator-API-based user creation (or deleted if folded into `token.go`'s CI path)
- `.github/workflows/ci.yml` — `functional-test` job gains emulator startup + JWKS publish-to-S3 steps before `sam deploy`; test-user creation step reworked
- `CONTRIBUTING.md` — references to `mockoidc`/Cognito updated
- `.mockery.yml` — unchanged (still mocks `tokenVerifier`, still needed for `OptionalAuth`'s tests)

---

### Task 1: Point `internal/auth` at a configurable OIDC issuer (no functional change yet)

This task is pure preparation — confirm `internal/auth.NewVerifier` already takes issuer/audience as parameters (it does, per `functions/api/config.go`'s `OIDC_ISSUER_URL`/`OIDC_AUDIENCE`), so no code change is needed here. Skip to Task 2. (Retained as a numbered task so the plan's task numbering stays stable across review — no-op, immediately proceed.)

---

### Task 2: Run the WorkOS emulator locally via Docker and confirm it boots

**Files:**
- Create: `scripts/workos-emulate-seed.yaml`
- Modify: `docker-compose.yml`

**Interfaces:**
- Produces: a running `workos-emulate` container on `docker-compose`'s default network, reachable at `http://workos-emulate:4100` from sibling containers and `http://localhost:4100` from the host, with a pre-seeded Connect OAuth application at a fixed `client_id`.

- [ ] **Step 1: Write the seed config**

```yaml
# scripts/workos-emulate-seed.yaml
users:
  - email: kbdb-local-test-user@rogueserenity.dev
    password: kbdb-local-test-password-1
  - email: kbdb-local-second-user@rogueserenity.dev
    password: kbdb-local-test-password-2

connectApplications:
  - name: kbdb-local
    type: oauth
    client_id: client_local_kbdb
    audience: client_local_kbdb
```

(This mirrors `mockoidc`'s current fixed `TestClientID`/`TestClientSecret`
convenience — see `test/functional/support/mockoidc/fixtures` — but for
WorkOS's shape. Two users match today's `fixtures.TestUserSubject`/
`fixtures.SecondUserSubject` two-identity pattern.)

- [ ] **Step 2: Add the `workos-emulate` service to `docker-compose.yml`, alongside (not yet replacing) `mockoidc`**

```yaml
  workos-emulate:
    image: ghcr.io/workos/emulate:latest
    environment:
      - WORKOS_EMULATE_ISSUER=http://workos-emulate:4100
    ports:
      - "4100:4100"
    volumes:
      - ./scripts/workos-emulate-seed.yaml:/app/workos-emulate.config.yaml:ro
```

- [ ] **Step 3: Boot it and confirm health/JWKS**

Run: `docker compose up -d workos-emulate`
Run: `sleep 3 && curl -sf http://localhost:4100/health`
Expected: `{"status":"ok"}`

Run: `curl -s http://localhost:4100/oauth2/jwks`
Expected: a JSON object with a non-empty `keys` array.

- [ ] **Step 4: Confirm the seeded user can authenticate via password grant**

Run:
```bash
curl -s -X POST http://localhost:4100/user_management/authenticate \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "client_local_kbdb",
    "client_secret": "sk_test_default",
    "grant_type": "password",
    "email": "kbdb-local-test-user@rogueserenity.dev",
    "password": "kbdb-local-test-password-1"
  }'
```
Expected: a JSON response containing `access_token` (a `.`-delimited JWT
string) and `user.email` matching the seeded email.

- [ ] **Step 5: Tear down**

Run: `docker compose down`

- [ ] **Step 6: Commit**

```bash
git add scripts/workos-emulate-seed.yaml docker-compose.yml
git commit -m "chore: add WorkOS emulator Docker service alongside mockoidc"
```

---

### Task 3: Rework `test/functional/support` to talk to the emulator instead of `mockoidc`

**Files:**
- Modify: `test/functional/support/env.go`
- Modify: `test/functional/support/api/token.go`
- Test: existing functional specs exercise this indirectly; no new test file — this task is verified by Task 4/9's specs passing, and by direct `go build`/`go vet` here since there's no unit-test layer for this support package today

**Interfaces:**
- Consumes: none new (same shape as today — HTTP calls to a configurable base URL)
- Produces: `support.EmulatorBaseURL() string`, `support.EmulatorClientID`/`EmulatorClientSecret` constants, `api.AuthToken(ctx) (string, error)`/`api.SecondUserAuthToken(ctx) (string, error)` — same signatures as today, so no caller elsewhere in `test/functional/features/` needs to change.

- [ ] **Step 1: Replace `env.go`'s mockoidc-specific functions**

Edit `test/functional/support/env.go`:

```go
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
// real-WorkOS-token path is separate - see api.AuthToken).
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

// DynamoDBEndpointURL returns the DynamoDB endpoint specs should talk to.
// Empty uses default AWS endpoint resolution.
func DynamoDBEndpointURL() string {
	return os.Getenv("KBDB_DYNAMODB_ENDPOINT_URL")
}
```

- [ ] **Step 2: Rework `token.go`'s token-minting to call the emulator's real HTTP API**

Edit `test/functional/support/api/token.go` — replace `QueueUser`/`MintToken` (mockoidc's bespoke queue-then-authorize-redirect dance) with direct emulator calls (create-or-reuse the user, then password-grant):

```go
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// postJSON POSTs body as application/json and decodes the response into
// out (if non-nil), returning an error (including the response body)
// unless the response status is exactly wantStatus.
func postJSON(ctx context.Context, url string, headers map[string]string, body []byte, wantStatus int, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request to %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response from %s: %w", url, err)
	}
	if resp.StatusCode != wantStatus {
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, respBody)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decoding response from %s: %w", url, err)
		}
	}
	return nil
}

// ensureEmulatorUser creates email/password as a WorkOS emulator user if it
// doesn't already exist. A 422 (already exists) is not an error - specs may
// call this repeatedly across a suite run.
func ensureEmulatorUser(ctx context.Context, email, password string) error {
	body, err := json.Marshal(map[string]any{
		"email":          email,
		"password":       password,
		"email_verified": true,
	})
	if err != nil {
		return fmt.Errorf("encoding create-user request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		support.EmulatorBaseURL()+"/user_management/users", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building create-user request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+support.EmulatorClientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling create-user: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnprocessableEntity {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create-user returned %d: %s", resp.StatusCode, respBody)
	}
	return nil
}

// mintEmulatorToken drives the WorkOS emulator's password grant and returns
// a real, signed access token for email/password.
func mintEmulatorToken(ctx context.Context, email, password string) (string, error) {
	if err := ensureEmulatorUser(ctx, email, password); err != nil {
		return "", err
	}

	body, err := json.Marshal(map[string]any{
		"client_id":     support.EmulatorClientID,
		"client_secret": support.EmulatorClientSecret,
		"grant_type":    "password",
		"email":         email,
		"password":      password,
	})
	if err != nil {
		return "", fmt.Errorf("encoding authenticate request: %w", err)
	}

	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	err = postJSON(ctx, support.EmulatorBaseURL()+"/user_management/authenticate", nil, body, http.StatusOK, &tokens)
	if err != nil {
		return "", err
	}
	if tokens.AccessToken == "" {
		return "", fmt.Errorf("authenticate response missing access_token")
	}
	return tokens.AccessToken, nil
}

// TokenSubject returns the "sub" claim of an access token, decoded without
// signature verification. Fine for functional tests: the caller already
// obtained this token from a trusted issuer (the WorkOS emulator or, in CI,
// real WorkOS) via AuthToken/SecondUserAuthToken - this just reads its
// subject back out. Needed because the emulator/real-WorkOS token path
// mints a real WorkOS-generated subject, not a fixed constant - specs that
// need to know "my own subject" (e.g. to seed owned fixture data) can't
// assume a fixed value.
func TokenSubject(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
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

// authToken mints a token for the given fixture identity via the WorkOS
// emulator, unless envOverride is set (a real WorkOS-minted token, when
// support.BaseURL() points at a real deployed stack instead) - a local
// emulator and real WorkOS aren't interchangeable token issuers, so this
// can't be derived from support.BaseURL() alone.
func authToken(ctx context.Context, envOverride, email, password string) (string, error) {
	if v := os.Getenv(envOverride); v != "" {
		return v, nil
	}
	return mintEmulatorToken(ctx, email, password)
}

// AuthToken returns a valid bearer token for the plain (non-admin) test
// user. See authToken for the KBDB_AUTH_TOKEN override behavior.
func AuthToken(ctx context.Context) (string, error) {
	return authToken(ctx, "KBDB_AUTH_TOKEN",
		"kbdb-local-test-user@rogueserenity.dev", "kbdb-local-test-password-1")
}

// SecondUserAuthToken returns a valid bearer token for a second, unrelated
// plain (non-admin) test user - distinct from AuthToken's identity, for
// exercising ownership/visibility-scoped reads of another user's items. See
// authToken for the KBDB_SECOND_USER_AUTH_TOKEN override behavior.
func SecondUserAuthToken(ctx context.Context) (string, error) {
	return authToken(ctx, "KBDB_SECOND_USER_AUTH_TOKEN",
		"kbdb-local-second-user@rogueserenity.dev", "kbdb-local-test-password-2")
}
```

Note: the admin/`cognito:groups` path (`QueueUser`'s `groups` parameter)
is dropped here — no functional spec currently exercises an admin token
through `AuthToken`/`SecondUserAuthToken` (grep confirms `groups` is only
ever passed as `nil` from `AuthToken`/`SecondUserAuthToken` themselves);
if a future admin-gated functional spec needs one, it's a separate,
later addition once kbdb has a WorkOS-side admin-group equivalent
designed — out of scope for this migration.

- [ ] **Step 2: Build to confirm no compile errors**

Run: `go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add test/functional/support/env.go test/functional/support/api/token.go
git commit -m "test: mint functional-test tokens via the WorkOS emulator's API"
```

---

### Task 4: Set up a real WorkOS Staging environment for kbdb (dashboard/API config, not code)

This task has no source-code deliverable — it's WorkOS dashboard/API
configuration, verified by direct API calls (as already done live during
design). Perform once per WorkOS environment kbdb needs (Staging now;
Production later, same steps).

**Interfaces:**
- Produces: a WorkOS User Management application's `client_id` (used as
  the native authorizer's `audience`), a first-party Connect OAuth
  application's `client_id` (used by real clients to register/authenticate),
  and the environment's AuthKit domain (used as the native authorizer's
  `issuer`) — three values later tasks consume.

- [ ] **Step 1: Confirm (or create) the User Management application**

Via the WorkOS dashboard or MCP tool (`environmentApplications`/
`defaultAuthkitApplication` queries) for the target environment. Record
its `client_id` — this is the `audience` value for Task 6.

- [ ] **Step 2: Create the first-party Connect OAuth application**

Via `createApplication` (no `organizationId`, `type: OAuth`,
`clientConfidentiality: Public`). Record its `client_id`.

- [ ] **Step 3: Set redirect URIs on the Connect application**

Via `setRedirectUris`: one non-wildcard default loopback URI (e.g.
`http://localhost:8787/callback`) plus `http://localhost:*/callback` as
a non-default wildcard entry (confirmed live: WorkOS rejects the wildcard
as the sole/default URI, both must be registered).

- [ ] **Step 4: Record the environment's AuthKit domain**

Via `dashboardSession` query — the `authkitDomains[].domain` value. This
is the `issuer` value for Task 6.

- [ ] **Step 5: No commit** — this task produces configuration values, not files. Record the three values (User Management `client_id`, Connect `client_id`, AuthKit domain) for use in Task 6.

---

### Task 5: Reconfigure `internal/auth`'s callers for WorkOS, drop `middleware.Auth`

**Files:**
- Modify: `internal/auth/verify.go` (adds `NewVerifierForTesting`, a
  test-support constructor — see Step 1)
- Modify: `internal/middleware/auth.go`
- Modify: `internal/router/router.go`
- Modify: `internal/mcp/server.go`
- Modify: `functions/api/config.go`
- Modify: `functions/api/main.go`
- Test: `internal/middleware/` has no existing test file for `Auth`/`OptionalAuth` (confirmed: only `recover_test.go` exists in that package) — this task adds one

**Interfaces:**
- Consumes: `auth.Verifier`/`auth.NewVerifier`/`auth.BearerToken` — unchanged signatures.
- Produces: `middleware.OptionalAuth(verifier *auth.Verifier) func(http.Handler) http.Handler` — same signature, now the package's only exported middleware function alongside `Logging`/`Recover`.

- [ ] **Step 1: Add a test-only constructor to `internal/auth` so external packages can inject a mock verifier**

`Verifier`'s `verifier tokenVerifier` field is unexported (by design —
see `CLAUDE.md`'s note on this dependency-injection pattern), and
`internal/auth/verify_test.go`'s existing tests construct
`&Verifier{verifier: s.mockVerifier}` from *inside* `package auth`,
which `internal/middleware`'s tests cannot do. Add a small exported
test-support constructor to `internal/auth` itself (`verify.go`, right
after `NewVerifier`):

```go
// NewVerifierForTesting constructs a Verifier from an already-configured
// tokenVerifier, for tests in other packages that need to inject a mock
// (internal/auth/mocks.MockTokenVerifier) without a real OIDC discovery
// round-trip. Not used by production code - see NewVerifier for that.
func NewVerifierForTesting(v tokenVerifier) *Verifier {
	return &Verifier{verifier: v}
}
```

Since `tokenVerifier` itself is unexported, only packages that already
import `internal/auth/mocks` (whose generated `MockTokenVerifier`
satisfies the unexported interface structurally, without needing to name
it) can call this — exactly `internal/middleware`'s new test, and no
wider exposure than that.

- [ ] **Step 2: Write the failing test for `OptionalAuth` (there isn't one yet — write it before touching `auth.go`, to lock in current behavior before refactoring)**

Create `internal/middleware/auth_test.go`:

```go
package middleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/auth/mocks"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/middleware"
)

type OptionalAuthSuite struct {
	suite.Suite

	mockVerifier *mocks.MockTokenVerifier
	verifier     *auth.Verifier
}

func TestOptionalAuthSuite(t *testing.T) {
	suite.Run(t, new(OptionalAuthSuite))
}

func (s *OptionalAuthSuite) SetupTest() {
	s.mockVerifier = mocks.NewMockTokenVerifier(s.T())
	s.verifier = auth.NewVerifierForTesting(s.mockVerifier)
}

func (s *OptionalAuthSuite) TestNoTokenProceedsAnonymously() {
	// mockVerifier has no EXPECT() set up - if OptionalAuth called
	// Verify at all in this case, the mock would fail the test on its
	// own (mockery's generated mocks fail on unexpected calls by
	// default), so this doubles as an assertion that VerifyToken is
	// never invoked for a request with no token.
	var calledWithUserID string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calledWithUserID, _ = ctxpkg.UserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	middleware.OptionalAuth(s.verifier)(next).ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Empty(calledWithUserID)
}

func (s *OptionalAuthSuite) TestInvalidTokenIsRejected() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "not-a-real-token").
		Return(nil, errors.New("oidc: signature verification failed"))
	nextCalled := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		nextCalled = true
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	middleware.OptionalAuth(s.verifier)(next).ServeHTTP(rec, req)

	s.Equal(http.StatusUnauthorized, rec.Code)
	s.False(nextCalled)
}

func (s *OptionalAuthSuite) TestValidTokenProceedsAuthenticated() {
	s.mockVerifier.EXPECT().
		Verify(mock.Anything, "a-valid-token").
		Return(&oidc.IDToken{Subject: "user-123"}, nil)
	var calledWithUserID string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calledWithUserID, _ = ctxpkg.UserID(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer a-valid-token")
	rec := httptest.NewRecorder()
	middleware.OptionalAuth(s.verifier)(next).ServeHTTP(rec, req)

	s.Equal(http.StatusOK, rec.Code)
	s.Equal("user-123", calledWithUserID)
}
```

Since Step 1 (adding `NewVerifierForTesting`) and this test are both
prerequisites of each other compiling, apply Step 1's code change now if
it isn't already applied, then continue.

- [ ] **Step 3: Run the test, confirm current behavior passes unchanged**

Run: `go test ./internal/middleware/... -run TestOptionalAuthSuite -v`
Expected: PASS (3/3) — this locks in `OptionalAuth`'s current behavior
before Step 4 removes `Auth()` from the same file, confirming the
removal doesn't accidentally change `OptionalAuth`.

- [ ] **Step 4: Remove `Auth()` from `internal/middleware/auth.go`, keep `OptionalAuth()`**

```go
package middleware

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/auth"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	logpkg "github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
)

// OptionalAuth verifies the bearer token if one is present, for routes that
// allow anonymous callers (e.g. reads on items whose visibility may permit
// public or authenticated-but-not-owner access — see
// [github.com/rogueserenity/kbdb/internal/authz]). A missing token is not
// an error and the request proceeds with no user ID on its context; a
// present-but-invalid token is still rejected with 401, since silently
// treating it as anonymous would hide a real client error.
//
// Required-auth routes no longer have an equivalent in-process middleware:
// they rely solely on API Gateway's native JWT authorizer (see
// template.yaml's HttpApi.Auth.DefaultAuthorizer), which verifies the
// identical token before the request ever reaches this process. That
// in-process re-verification was deliberate defense-in-depth under
// Cognito; re-examined for the WorkOS migration and dropped as
// redundant — the native authorizer is the same class of AWS-verified
// mechanism Cognito's JWT authorizer already was.
func OptionalAuth(verifier *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := auth.BearerToken(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			authedReq, err := authenticate(r, verifier, rawToken)
			if err != nil {
				problem.Unauthorized(w, "invalid token")
				return
			}

			next.ServeHTTP(w, authedReq)
		})
	}
}

// authenticate verifies rawToken and returns r with its context updated to
// carry the verified caller's user ID, groups, and request-scoped logger.
func authenticate(r *http.Request, verifier *auth.Verifier, rawToken string) (*http.Request, error) {
	claims, err := verifier.VerifyToken(r.Context(), rawToken)
	if err != nil {
		// Warn, not Error: an individual invalid/expired token from one
		// client is expected traffic, not a bug - still worth a trace to
		// spot a misconfigured client or repeated probing.
		logpkg.FromContext(r.Context()).Warn("token verification failed", logpkg.Error, err)
		return nil, err
	}

	ctx := ctxpkg.WithUserID(r.Context(), claims.Subject)
	ctx = ctxpkg.WithGroups(ctx, claims.Groups)
	l := logpkg.WithUserID(logpkg.FromContext(ctx), claims.Subject)
	ctx = logpkg.WithLogger(ctx, l)

	return r.WithContext(ctx), nil
}
```

- [ ] **Step 5: Confirm the test still passes after `Auth()`'s removal**

Run: `go test ./internal/middleware/... -run TestOptionalAuthSuite -v`
Expected: PASS (3/3) — `OptionalAuth`'s behavior is unaffected by
removing the unrelated `Auth()` function from the same file.

- [ ] **Step 6: Remove `middleware.Auth(verifier)(...)` wrapping in `internal/router/router.go`**

For every one of the 14 required-auth call sites (all `POST`/`PUT`/`DELETE`
routes across switches, keyboards, keycap-sets, builds), change:

```go
mux.Handle("POST /v1/users/{userId}/switches",
    middleware.Auth(verifier)(validate(handlers.CreateSwitch(switchRepo))))
```

to:

```go
mux.Handle("POST /v1/users/{userId}/switches",
    validate(handlers.CreateSwitch(switchRepo)))
```

Apply the identical transformation to all 14 sites: `CreateSwitch`,
`UpdateSwitch`, `DeleteSwitch`, `CreateKeyboard`, `UpdateKeyboard`,
`DeleteKeyboard`, `CreateKeycapSet`, `UpdateKeycapSet`, `DeleteKeycapSet`,
`CreateKeycapKit`, `UpdateKeycapKit`, `DeleteKeycapKit`,
`SetKeycapKitImage`, `DeleteKeycapKitImage`, `CreateBuild`, `UpdateBuild`,
`DeleteBuild`, `AddBuildImage`, `DeleteBuildImage` (19 handlers total —
recount against the actual file, this list is illustrative of the
pattern, not exhaustive; every `middleware.Auth(verifier)(...)` call site
in the file gets this same transformation, `middleware.OptionalAuth(...)`
sites are untouched).

- [ ] **Step 7: Remove `requireBearerToken`/`tokenVerifier` wrapping in `internal/mcp/server.go`**

Change:
```go
return Handlers{
    Streamable:       requireBearerToken(verifier, streamable),
    MetadataPath:     MetadataPath,
    RootMetadataPath: RootMetadataPath,
    Metadata:         metadataHandler(issuerURL),
}
```
to:
```go
return Handlers{
    Streamable:       streamable,
    MetadataPath:     MetadataPath,
    RootMetadataPath: RootMetadataPath,
    Metadata:         metadataHandler(issuerURL),
}
```

Delete the now-unused `requireBearerToken` and `tokenVerifier` functions
entirely from `server.go`. Check `internal/mcp/server.go`'s imports for
`sdkauth` — if `requireBearerToken`/`tokenVerifier` were its only
consumers, remove the import too. `New`'s `verifier *auth.Verifier`
parameter is now unused inside `server.go` itself — remove it from `New`'s
signature, and update its one call site in `internal/router/router.go`
(`mcp.New(verifier, ...)` → `mcp.New(...)`, dropping the first argument)
accordingly. Check `internal/mcp/*_test.go` for any test construction of
`mcp.New(...)` that also needs its `verifier` argument dropped.

- [ ] **Step 8: Update `functions/api/config.go`'s env var names/comments to reflect WorkOS**

```go
package main

// Config is populated from environment variables via Kong. There are no CLI
// flags for this service — it's a Lambda entrypoint, not a CLI tool — but
// Kong's struct-tag env binding, defaults, and required-field validation are
// useful regardless of whether flag parsing is ever exercised.
type Config struct {
	// OIDCIssuerURL/OIDCAudience configure the verifier used only by
	// middleware.OptionalAuth (required-auth routes rely solely on API
	// Gateway's native JWT authorizer - see template.yaml). Points at
	// WorkOS Connect's issuer (the environment's AuthKit domain) and the
	// WorkOS User Management application's client_id, respectively.
	OIDCIssuerURL      string `env:"OIDC_ISSUER_URL" required:""`
	OIDCAudience       string `env:"OIDC_AUDIENCE" required:""`
	ImagesBucketName   string `env:"IMAGES_BUCKET_NAME" required:""`
	SwitchTableName    string `env:"SWITCH_TABLE_NAME" required:""`
	KeyboardTableName  string `env:"KEYBOARD_TABLE_NAME" required:""`
	KeycapSetTableName string `env:"KEYCAP_SET_TABLE_NAME" required:""`
	BuildTableName     string `env:"BUILD_TABLE_NAME" required:""`

	// Empty in real deployments; set locally to point at LocalStack.
	DynamoDBEndpointURL string `env:"DYNAMODB_ENDPOINT_URL"`

	// Empty in real deployments; set locally to point at LocalStack.
	S3EndpointURL string `env:"S3_ENDPOINT_URL"`
}
```

(Env var *names* stay the same — only the comment and, in Task 6, the
values they're populated with in `template.yaml` change — so
`functions/api/main.go`'s `auth.NewVerifier(ctx, cfg.OIDCIssuerURL,
cfg.OIDCAudience)` call needs no code change, only Task 6's deployed
values change.)

- [ ] **Step 9: Build and run all existing unit tests**

Run: `mise run test`
Expected: builds cleanly; all existing tests pass (some `internal/mcp`
tests may need the `mcp.New(...)` call-site argument-count fix from Step
7 — fix any remaining compile errors here before proceeding).

- [ ] **Step 10: Commit**

```bash
git add internal/auth/verify.go internal/middleware/auth.go \
        internal/middleware/auth_test.go internal/router/router.go \
        internal/mcp/server.go functions/api/config.go
git commit -m "refactor: drop in-process auth for required-auth routes, keep OptionalAuth"
```

---

### Task 6: Swap `template.yaml`'s authorizer from Cognito to WorkOS, remove Cognito resources

**Files:**
- Modify: `template.yaml`

**Interfaces:**
- Consumes: the three values recorded in Task 4 (User Management
  `client_id`, Connect `client_id`, AuthKit domain).
- Produces: `HttpApi.Auth.DefaultAuthorizer` pointing at a
  `WorkOSAuthorizer` JWT authorizer definition; `OIDC_ISSUER_URL`/
  `OIDC_AUDIENCE` env vars populated from WorkOS values via SAM
  `Parameters` (not hardcoded — different per deployed stack, same
  reasoning `UserPool`/`UserPoolClient` had for being per-stack resources).

- [ ] **Step 1: Add SAM `Parameters` for the WorkOS values**

Near the top of `template.yaml` (alongside any existing `Parameters`
block — check the file for one; add a new `Parameters:` top-level key if
none exists yet):

```yaml
Parameters:
  WorkOSAuthKitDomain:
    Type: String
    Description: >-
      This deployment's WorkOS AuthKit domain (the Connect issuer),
      e.g. a *.authkit.app subdomain or a configured custom domain.
  WorkOSUserManagementClientId:
    Type: String
    Description: >-
      The WorkOS User Management application's client_id for this
      environment - used as the native JWT authorizer's audience,
      since every Connect-issued access token's aud claim resolves to
      this value (confirmed empirically, not derived from any
      resource= parameter a client supplies).
```

- [ ] **Step 2: Replace `CognitoAuthorizer` with a native WorkOS JWT authorizer**

Change:
```yaml
  HttpApi:
    Type: AWS::Serverless::HttpApi
    Properties:
      Auth:
        Authorizers:
          CognitoAuthorizer:
            JwtConfiguration:
              issuer: !Sub https://cognito-idp.${AWS::Region}.amazonaws.com/${UserPool}
              audience:
                - !Ref UserPoolClient
            IdentitySource: $request.header.Authorization
        DefaultAuthorizer: CognitoAuthorizer
```
to:
```yaml
  HttpApi:
    Type: AWS::Serverless::HttpApi
    Properties:
      Auth:
        Authorizers:
          WorkOSAuthorizer:
            JwtConfiguration:
              issuer: !Sub https://${WorkOSAuthKitDomain}
              audience:
                - !Ref WorkOSUserManagementClientId
            IdentitySource: $request.header.Authorization
        DefaultAuthorizer: WorkOSAuthorizer
```

- [ ] **Step 3: Delete the `UserPool`, `UserPoolClient`, `UserPoolDomain` resources entirely**

Remove all three `AWS::Cognito::*` resource blocks from `template.yaml`.

- [ ] **Step 4: Delete the Cognito-related `Outputs`**

Remove `UserPoolId`, `UserPoolClientId`, `UserPoolHostedUiDomain` from
the `Outputs:` block.

- [ ] **Step 5: Update `ApiFunction`'s environment variables**

Change:
```yaml
          OIDC_ISSUER_URL: !Sub https://cognito-idp.${AWS::Region}.amazonaws.com/${UserPool}
          OIDC_AUDIENCE: !Ref UserPoolClient
```
to:
```yaml
          OIDC_ISSUER_URL: !Sub https://${WorkOSAuthKitDomain}
          OIDC_AUDIENCE: !Ref WorkOSUserManagementClientId
```

- [ ] **Step 6: Remove `Authorizer: NONE` from every required-auth REST route and all three MCP routes' `Auth` blocks; keep it only on the two lookup GET routes**

For every `Auth: { Authorizer: NONE }` block currently on a
required-auth route (the ones whose comment reads "Default
CognitoAuthorizer applies") — these already have no `Auth:` override at
all in the current file (confirmed: they inherit `DefaultAuthorizer`
implicitly, per the existing comments) — no change needed for those.

For the three MCP routes (`McpEvent`, `McpMetadataEvent`,
`McpMetadataMcpEvent`) and any optional-auth REST GET route currently
marked `Authorizer: NONE`: **MCP routes lose their `Authorizer: NONE`
override** (they now pick up `DefaultAuthorizer`/`WorkOSAuthorizer`); **REST
GET optional-auth routes and the two lookup routes keep
`Authorizer: NONE` unchanged.**

Remove `Authorizer: NONE` from `McpEvent`, `McpMetadataEvent`,
`McpMetadataMcpEvent`'s `Auth:` blocks entirely (delete the `Auth:` key
from each of those three route Events).

- [ ] **Step 7: Update the template's top-level description comment**

Change line 5's "Lambda function fronted by an HTTP API with a Cognito
JWT authorizer" to "Lambda function fronted by an HTTP API with a WorkOS
JWT authorizer".

- [ ] **Step 8: Update stale `# Default CognitoAuthorizer applies` / `# Auth: NONE at the gateway` comments throughout the route Events**

Replace every occurrence of "CognitoAuthorizer" in a comment with
"WorkOSAuthorizer"; the "Auth: NONE at the gateway (security: [{}, ...]
in api/openapi.yaml..." comments on optional-auth REST GET routes and
the fully-public lookup routes stay accurate as-is (mechanism unchanged),
just reference the same authorizer name update where they name it.

- [ ] **Step 9: Validate the template**

Run: `sam validate --lint`
Expected: no errors. (This does not require real WorkOS values —
`Parameters` without a `Default` just need to be syntactically valid
placeholders for `sam validate`; real values are supplied at
`sam deploy` time via `--parameter-overrides`.)

- [ ] **Step 10: Commit**

```bash
git add template.yaml
git commit -m "feat: swap Cognito authorizer for native WorkOS JWT authorizer"
```

---

### Task 7: Wire WorkOS parameters into `dev-deploy.sh` and `mise.toml`'s dev tasks

**Files:**
- Modify: `scripts/dev-deploy.sh`
- Modify: `mise.toml` (only if `dev-setup`/`dev-deploy` task descriptions reference Cognito)

**Interfaces:**
- Consumes: `KBDB_WORKOS_AUTHKIT_DOMAIN`/`KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID` env vars (new — a developer sets these once per personal WorkOS environment, same spirit as `KBDB_DEV_NAME`/`KBDB_DEV_REGION`).

- [ ] **Step 1: Add parameter overrides to `dev-deploy.sh`**

Edit `scripts/dev-deploy.sh`, adding after the existing variable
declarations:

```bash
WORKOS_AUTHKIT_DOMAIN="${KBDB_WORKOS_AUTHKIT_DOMAIN:?set KBDB_WORKOS_AUTHKIT_DOMAIN - see CONTRIBUTING.md}"
WORKOS_USER_MANAGEMENT_CLIENT_ID="${KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID:?set KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID - see CONTRIBUTING.md}"
```

and adding to the `sam deploy` invocation's argument list:
```bash
  --parameter-overrides "SkipApiRepository=true" \
    "WorkOSAuthKitDomain=${WORKOS_AUTHKIT_DOMAIN}" \
    "WorkOSUserManagementClientId=${WORKOS_USER_MANAGEMENT_CLIENT_ID}" \
```

(Check the file's actual current `--parameter-overrides` line — there
may not be an existing `SkipApiRepository=true` override in
`dev-deploy.sh` specifically, that was from `ci.yml`; add the two new
overrides as the full `--parameter-overrides` value if none exists yet,
following whatever pattern the file already uses for
`--image-repositories`/other per-run flags.)

- [ ] **Step 2: Deploy your own dev stack to confirm the parameters flow through**

Run: `KBDB_WORKOS_AUTHKIT_DOMAIN=<your Task 4 value> KBDB_WORKOS_USER_MANAGEMENT_CLIENT_ID=<your Task 4 value> mise run dev-deploy`
Expected: deploy succeeds; `aws cloudformation describe-stacks --stack-name kbdb-dev-<you> --query "Stacks[0].Parameters"` shows both new parameters set correctly.

- [ ] **Step 3: Commit**

```bash
git add scripts/dev-deploy.sh
git commit -m "feat: wire WorkOS parameters into dev-deploy.sh"
```

---

### Task 8: Confirm a real end-to-end request against your dev stack

This task has no code changes — it's the first real validation that
Tasks 5-7 actually work together against real AWS + real WorkOS.

- [ ] **Step 1: Mint a real WorkOS Connect token for your dev stack's Connect application**

Using the same PKCE + `/oauth2/authorize` + `/oauth2/token` flow already
proven live during design (see the spec's "WorkOS architecture" section
for the exact request shapes), against your dev stack's Connect
`client_id`.

- [ ] **Step 2: Call a required-auth REST route with the token**

Run: `curl -H "Authorization: Bearer <token>" https://<your dev API URL>/v1/users/<your sub>/switches -X POST -d '{...}'`
Expected: a real `201`/successful response — not a 401 from the
authorizer.

- [ ] **Step 3: Call the same route with no token**

Run: `curl https://<your dev API URL>/v1/users/<your sub>/switches -X POST -d '{...}'`
Expected: `401` — confirms the native authorizer, not just app code, is
actually gating the route (no `Authorizer: NONE` fallback left in place
by mistake).

- [ ] **Step 4: Call `/mcp` with the token**

Confirm the MCP endpoint also requires and accepts the same token
(a basic MCP `initialize` handshake, per existing functional-test
patterns in `test/functional/features/mcp/`).

- [ ] **Step 5: No commit** — this is a manual verification task, not a code change.

---

### Task 9: Replace `mockoidc` in local dev tooling

**Files:**
- Delete: `test/functional/support/mockoidc/` (entire directory)
- Modify: `docker-compose.yml`
- Modify: `test/functional/support/env.local.json`
- Modify: `scripts/func-setup.sh`
- Modify: `CONTRIBUTING.md`

**Interfaces:**
- Produces: `mise run func-setup` brings up `workos-emulate` (not
  `mockoidc`) alongside `localstack`, with `sam local start-api`
  configured to verify against it.

- [ ] **Step 1: Remove the `mockoidc` service from `docker-compose.yml`, keep `workos-emulate` (added in Task 2)**

Delete the `mockoidc:` service block entirely from `docker-compose.yml`.

- [ ] **Step 2: Delete `test/functional/support/mockoidc/` entirely**

Run: `git rm -r test/functional/support/mockoidc`

- [ ] **Step 3: Update `env.local.json`'s OIDC values**

Change:
```json
    "OIDC_ISSUER_URL": "http://mockoidc:9999/oidc",
    "OIDC_AUDIENCE": "no-client-id-here-ok",
```
to:
```json
    "OIDC_ISSUER_URL": "http://workos-emulate:4100",
    "OIDC_AUDIENCE": "client_local_kbdb",
```

- [ ] **Step 4: Update `scripts/func-setup.sh`'s comment references to mockoidc, if any**

Grep the file for "mockoidc" and replace any comment mentions with
"workos-emulate" — the script's actual logic (LocalStack table
provisioning, `sam local start-api` invocation) needs no functional
change, since `docker-compose up -d --build` already brings up whatever
services are defined in `docker-compose.yml`.

- [ ] **Step 5: Update `CONTRIBUTING.md`**

Change:
```
mise run func-setup    # brings up LocalStack + mockoidc + sam local start-api
```
to:
```
mise run func-setup    # brings up LocalStack + the WorkOS emulator + sam local start-api
```

Change:
```
`mockoidc` stands in for Cognito in functional tests, since `auth.NewVerifier` does a real OIDC discovery round-trip and needs something real to talk to. It runs as its own docker-compose service; see `test/functional/support/mockoidc/main.go` if you need to touch it.
```
to:
```
The WorkOS emulator (`ghcr.io/workos/emulate`) stands in for WorkOS in functional tests, since `auth.NewVerifier` does a real OIDC discovery round-trip and needs something real to talk to. It runs as its own docker-compose service, seeded via `scripts/workos-emulate-seed.yaml`.
```

- [ ] **Step 6: Full local functional run**

Run: `mise run func-teardown` (clean slate)
Run: `mise run func-setup`
Run: `mise run func-test`
Expected: the full Ginkgo suite passes, including auth-dependent specs
(required-auth routes now gated by `sam local start-api`'s own
enforcement of `template.yaml`'s authorizer config — confirm
`sam local start-api` actually respects `AuthorizerType: JWT` config
during Step 6's run; if it does not — SAM CLI's local emulation of native
JWT authorizers has historically been incomplete — note this as a known
gap and fall back to relying on the optional-auth in-process path plus
Task 8's real-AWS confirmation as the actual authorizer coverage, since
this is a SAM CLI limitation, not a kbdb design gap).

- [ ] **Step 7: Commit**

```bash
git add docker-compose.yml test/functional/support/env.local.json \
        scripts/func-setup.sh CONTRIBUTING.md
git rm -r test/functional/support/mockoidc
git commit -m "chore: replace mockoidc with the WorkOS emulator in local dev"
```

---

### Task 10: Build the CI static-JWKS-to-S3 publishing script

**Files:**
- Create: `scripts/ci-publish-emulator-jwks.sh`

**Interfaces:**
- Consumes: an RSA private key (generated fresh each run — no need to
  persist across runs, since CI always republishes before every deploy).
- Produces: two objects in a designated S3 bucket/prefix: an OIDC
  discovery document and a JWKS document, at a stable, predictable URL
  path; prints the resulting issuer URL to stdout for the calling
  workflow step to capture.

- [ ] **Step 1: Write the script**

```bash
#!/usr/bin/env bash
set -euo pipefail
# Generates a throwaway RSA keypair, derives the corresponding static
# OIDC discovery document + JWKS, and publishes both to S3 at a stable
# URL - so a real deployed AWS stack's native JWT authorizer has a
# publicly-reachable issuer/jwks_uri to fetch from, without needing to
# reach back into the ephemeral GitHub Actions runner (which has no
# stable public IP/DNS - see docs/superpowers/specs/2026-08-16-workos-auth-migration-design.md's
# Testing section for the full reachability rationale).
#
# Usage: ci-publish-emulator-jwks.sh <s3-bucket> <s3-prefix>
# Prints: <issuer-url> <private-key-path> to stdout, space-separated,
# for the caller to capture and pass to `sam deploy` / the emulator.

S3_BUCKET="$1"
S3_PREFIX="$2"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

openssl genrsa -out "$WORKDIR/key.pem" 2048 >/dev/null 2>&1
openssl rsa -in "$WORKDIR/key.pem" -pubout -out "$WORKDIR/key.pub.pem" >/dev/null 2>&1

ISSUER_URL="https://${S3_BUCKET}.s3.amazonaws.com/${S3_PREFIX}"

# Derive kid (a short, stable identifier for this run's key) and the
# JWK's n/e (RSA modulus/exponent, base64url-no-padding) from the public
# key via openssl + python3 (both already required by this repo's CI/dev
# environment).
python3 - "$WORKDIR/key.pub.pem" "$ISSUER_URL" "$WORKDIR" <<'PYEOF'
import sys, base64, json
from cryptography.hazmat.primitives import serialization

pub_path, issuer, workdir = sys.argv[1], sys.argv[2], sys.argv[3]

with open(pub_path, "rb") as f:
    pub = serialization.load_pem_public_key(f.read())
numbers = pub.public_numbers()

def b64url(i: int) -> str:
    b = i.to_bytes((i.bit_length() + 7) // 8, "big")
    return base64.urlsafe_b64encode(b).rstrip(b"=").decode()

kid = "ci-" + base64.urlsafe_b64encode(str(numbers.n)[:16].encode()).rstrip(b"=").decode()[:16]

jwks = {
    "keys": [{
        "kty": "RSA",
        "use": "sig",
        "alg": "RS256",
        "kid": kid,
        "n": b64url(numbers.n),
        "e": b64url(numbers.e),
    }]
}

discovery = {
    "issuer": issuer,
    "jwks_uri": issuer + "/jwks.json",
    "authorization_endpoint": issuer + "/authorize",
    "token_endpoint": issuer + "/token",
    "response_types_supported": ["code"],
    "subject_types_supported": ["public"],
    "id_token_signing_alg_values_supported": ["RS256"],
}

with open(f"{workdir}/jwks.json", "w") as f:
    json.dump(jwks, f)
with open(f"{workdir}/openid-configuration", "w") as f:
    json.dump(discovery, f)
with open(f"{workdir}/kid", "w") as f:
    f.write(kid)
PYEOF

KID="$(cat "$WORKDIR/kid")"

aws s3 cp "$WORKDIR/jwks.json" "s3://${S3_BUCKET}/${S3_PREFIX}/jwks.json" \
  --content-type application/json --acl public-read >/dev/null
aws s3 cp "$WORKDIR/openid-configuration" "s3://${S3_BUCKET}/${S3_PREFIX}/.well-known/openid-configuration" \
  --content-type application/json --acl public-read >/dev/null

echo "${ISSUER_URL} ${WORKDIR}/key.pem ${KID}"
```

Note: this script derives JWKS from the public key directly (via
`cryptography`'s Python bindings — confirm `python3 -c "import cryptography"`
succeeds in the CI runner image; `ubuntu-24.04-arm` per `ci.yml` ships
Python 3 but `cryptography` may need `pip install cryptography` as a
prior step — add that if the Step 2 dry run fails on the import). This
avoids needing the emulator container running at all just to compute its
own JWKS — the whole point of pinning a key.

- [ ] **Step 2: Dry-run it against a scratch S3 location**

Run: `chmod +x scripts/ci-publish-emulator-jwks.sh`
Run: `pip3 install --user cryptography` (if the Step 1 note's import check failed)
Run: `./scripts/ci-publish-emulator-jwks.sh <a scratch bucket you control> ci-jwks-test-$(date +%s)`
Expected: prints `<issuer-url> <key-path> <kid>`; `curl <issuer-url>/jwks.json` and `curl <issuer-url>/.well-known/openid-configuration` both return valid JSON matching the script's output.

- [ ] **Step 3: Confirm the emulator can actually be pinned to the derived key and issuer, and that a token it mints verifies against the published JWKS**

Run:
```bash
docker run --rm -d -p 4100:4100 \
  -e WORKOS_EMULATE_ISSUER="<issuer-url from Step 2>" \
  -v <key-path from Step 2>:/keys/signing-key.pem:ro \
  ghcr.io/workos/emulate --signing-key /keys/signing-key.pem
sleep 3
curl -s http://localhost:4100/oauth2/jwks | python3 -m json.tool
```
Expected: the `kid` in this output matches the `kid` printed by Step 2's
script run — confirming the pinned key produces the exact same JWKS the
script already published to S3, without the emulator needing to be
reachable from anywhere but the CI runner itself.

- [ ] **Step 4: Commit**

```bash
git add scripts/ci-publish-emulator-jwks.sh
git commit -m "feat: add CI script to publish static emulator JWKS to S3"
```

---

### Task 11: Wire the JWKS-publish + emulator-token-minting steps into CI

**Files:**
- Modify: `.github/workflows/ci.yml`
- Delete: `scripts/ci-create-test-user.sh` (replaced)

**Interfaces:**
- Produces: `functional-test` job env vars `KBDB_AUTH_TOKEN`/
  `KBDB_SECOND_USER_AUTH_TOKEN` populated from emulator-minted tokens
  (same env var names Task 3's `token.go` already reads via
  `authToken`'s `envOverride` parameter — no `token.go` changes needed
  here).

- [ ] **Step 1: Add an S3 bucket parameter/reference for JWKS publishing**

Check whether `kbdb-sam-artifacts-475976462467` (the existing CI
artifacts bucket, per `samconfig.toml`'s `[ci.deploy.parameters]`) is
suitable to also hold the published JWKS under a per-PR-stack prefix, or
whether a dedicated bucket is warranted. Reuse the existing artifacts
bucket with prefix `jwks/kbdb-pr-<N>/` unless it lacks public-read
capability (check via `aws s3api get-bucket-policy` /
`get-public-access-block` — if `BlockPublicAcls`/`BlockPublicPolicy` are
enabled account-wide, either a separate public bucket is needed, or
switch the script's `--acl public-read` approach to presigned URLs with
a long expiry instead — confirm which before proceeding, this is a real
environment-specific fork in the plan, not a placeholder).

- [ ] **Step 2: Add a step to start the emulator and publish JWKS, before `sam deploy`**

Insert into `.github/workflows/ci.yml`'s `functional-test` job, after
`sam build` and before `sam deploy`:

```yaml
      - name: Publish emulator JWKS to S3
        id: emulator-jwks
        run: |
          read -r ISSUER_URL KEY_PATH KID < <(scripts/ci-publish-emulator-jwks.sh \
            kbdb-sam-artifacts-475976462467 "jwks/kbdb-pr-${{ github.event.pull_request.number }}")
          echo "issuer-url=$ISSUER_URL" >> "$GITHUB_OUTPUT"
          echo "key-path=$KEY_PATH" >> "$GITHUB_OUTPUT"

      - name: Start WorkOS emulator
        run: |
          docker run --rm -d -p 4100:4100 --name workos-emulate \
            -e WORKOS_EMULATE_ISSUER="${{ steps.emulator-jwks.outputs.issuer-url }}" \
            -v "${{ steps.emulator-jwks.outputs.key-path }}:/keys/signing-key.pem:ro" \
            ghcr.io/workos/emulate --signing-key /keys/signing-key.pem
          for i in $(seq 1 20); do
            curl -sf http://localhost:4100/health && break
            sleep 0.5
          done
```

- [ ] **Step 3: Update the `sam deploy` step to pass the WorkOS parameters**

Change the existing `sam deploy` invocation's `--parameter-overrides` to
add:
```yaml
            --parameter-overrides "SkipApiRepository=true" \
              "WorkOSAuthKitDomain=$(echo '${{ steps.emulator-jwks.outputs.issuer-url }}' | sed 's|^https://||')" \
              "WorkOSUserManagementClientId=client_local_kbdb" \
```

(`WorkOSAuthKitDomain` in `template.yaml` is used as `!Sub
https://${WorkOSAuthKitDomain}` — strip the `https://` prefix here since
the parameter is the bare domain, not a full URL; `client_local_kbdb` is
the emulator's fixed seeded client_id, matching Task 2's seed config —
CI's emulator doesn't need a seed *file* the way local dev's does, since
it only needs to mint tokens with a matching `aud`, achievable via the
emulator's default `sk_test_default` API key and a plain
`client_id`/`audience` implied by the token-minting call in Step 4 below,
not a pre-seeded Connect application — confirm this against Task 10
Step 3's dry run: does a token minted with no seeded Connect application
carry a usable `aud`? If not, mount a seed file here too, following
Task 2's pattern.)

- [ ] **Step 4: Replace the "Create throwaway Cognito test users" step**

Change:
```yaml
      - name: Create throwaway Cognito test users and mint tokens
        id: auth-token
        run: |
          USER_POOL_ID="${{ steps.stack-outputs.outputs.user-pool-id }}"
          USER_POOL_CLIENT_ID="${{ steps.stack-outputs.outputs.user-pool-client-id }}"
          PASSWORD="$(openssl rand -base64 24)Aa1!"

          TOKEN=$(scripts/ci-create-test-user.sh "$USER_POOL_ID" "$USER_POOL_CLIENT_ID" \
            "ci-test-user@rogueserenity.dev" "$PASSWORD")
          SECOND_USER_TOKEN=$(scripts/ci-create-test-user.sh "$USER_POOL_ID" "$USER_POOL_CLIENT_ID" \
            "ci-test-user-2@rogueserenity.dev" "$PASSWORD")

          echo "::add-mask::$TOKEN"
          echo "::add-mask::$SECOND_USER_TOKEN"
          {
            echo "token=$TOKEN"
            echo "second-user-token=$SECOND_USER_TOKEN"
          } >> "$GITHUB_OUTPUT"
```
to:
```yaml
      - name: Create test users and mint emulator tokens
        id: auth-token
        run: |
          PASSWORD="$(openssl rand -base64 24)Aa1!"

          create_and_mint() {
            local email="$1"
            curl -sf -X POST http://localhost:4100/user_management/users \
              -H "Content-Type: application/json" \
              -H "Authorization: Bearer sk_test_default" \
              -d "{\"email\":\"$email\",\"password\":\"$PASSWORD\",\"email_verified\":true}" >/dev/null

            curl -sf -X POST http://localhost:4100/user_management/authenticate \
              -H "Content-Type: application/json" \
              -d "{\"client_id\":\"client_local_kbdb\",\"client_secret\":\"sk_test_default\",\"grant_type\":\"password\",\"email\":\"$email\",\"password\":\"$PASSWORD\"}" \
              | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])"
          }

          TOKEN=$(create_and_mint "ci-test-user@rogueserenity.dev")
          SECOND_USER_TOKEN=$(create_and_mint "ci-test-user-2@rogueserenity.dev")

          echo "::add-mask::$TOKEN"
          echo "::add-mask::$SECOND_USER_TOKEN"
          {
            echo "token=$TOKEN"
            echo "second-user-token=$SECOND_USER_TOKEN"
          } >> "$GITHUB_OUTPUT"
```

Remove the now-unused `user-pool-id`/`user-pool-client-id` outputs from
the earlier "Read stack outputs" step (they referenced
`template.yaml` Outputs deleted in Task 6 Step 4, and would fail the
`aws cloudformation describe-stacks --query` call with an empty result
otherwise — confirm this step's `USER_POOL_ID`/`USER_POOL_CLIENT_ID`
lines are removed, not just unused).

- [ ] **Step 5: Delete `scripts/ci-create-test-user.sh`**

Run: `git rm scripts/ci-create-test-user.sh`

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/ci.yml
git rm scripts/ci-create-test-user.sh
git commit -m "ci: publish static emulator JWKS to S3, mint tokens via emulator API"
```

---

### Task 12: Open a real PR and confirm CI passes end-to-end

This task has no code changes of its own — it's the integration
checkpoint for everything in Tasks 1-11 together, against CI's actual
constraints (ephemeral runner, real deployed AWS stack, real network
boundaries) rather than any local approximation of them.

- [ ] **Step 1: Push the branch and open a PR**

- [ ] **Step 2: Watch the `functional-test` job**

Expected: `sam deploy` succeeds with the WorkOS parameters; the emulator
container starts and passes its health check; JWKS publish-to-S3
succeeds; test user creation + token minting succeeds; the full Ginkgo
suite passes against the real deployed stack, including specs that
exercise required-auth routes (now gated purely by the native
authorizer against the published static JWKS) and optional-auth routes
(gated by the in-process `OptionalAuth` path).

- [ ] **Step 3: If the `functional-test` job fails at the authorizer step specifically (401s on routes that should succeed)**

Diagnose via `aws apigatewayv2 get-authorizers --api-id <id>` against the
CI-deployed stack to confirm the authorizer's actual `issuer`/`audience`
match what was published; check API Gateway's authorizer JWKS cache TTL
(up to 2 hours per AWS docs, confirmed during design research) isn't
serving a stale cached JWKS from an earlier failed run against the same
stack name — `kbdb-pr-<N>` stacks are reused across pushes to the same
PR (per `ci-create-test-user.sh`'s original comment about this), so a
JWKS mismatch from an earlier run could persist; if this happens, note it
as a real operational gotcha to document in `CONTRIBUTING.md`, not
something to silently work around.

- [ ] **Step 4: No commit for this task** — fix any issues discovered here
by returning to the relevant earlier task and amending it, then re-push.

---

## Self-Review Notes

**Spec coverage**: Task 6 covers the authorizer swap + Cognito removal
(spec's "What kbdb builds" §1-2). Task 4 covers WorkOS configuration
(§2). Tasks 2-3, 9 cover the local-emulator testing design. Tasks 10-11
cover the CI static-JWKS design, including the documented fallback
consideration surfaced explicitly in Task 12 Step 3 rather than silently
assumed away. Task 5 covers the `internal/auth` consequence section,
including the defense-in-depth re-examination the spec documents.

**Known open decision points carried into specific tasks** (not
placeholders — genuine environment-specific forks flagged inline where
the actual answer depends on infrastructure this plan can't observe
ahead of execution): Task 11 Step 1 (S3 bucket public-read capability),
Task 11 Step 3 (whether CI's emulator needs a seed file), Task 5 Step 1
(exact zero-value-vs-mock pattern for the new test), Task 9 Step 6
(whether `sam local start-api` actually enforces native JWT authorizer
config).
