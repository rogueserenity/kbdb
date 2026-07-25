package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/mockoidc/fixtures"
)

// QueueUser sets which identity the next /oidc/authorize call on the
// mockoidc instance at issuerBaseURL (host:port, no /oidc suffix) should
// authenticate as, via its /test/queue-user control endpoint (see
// test/functional/support/mockoidc/main.go). groups may be nil for a
// non-admin user.
func QueueUser(ctx context.Context, issuerBaseURL, subject string, groups []string) error {
	body, err := json.Marshal(map[string]any{"subject": subject, "groups": groups})
	if err != nil {
		return fmt.Errorf("encoding queue-user request: %w", err)
	}

	return postJSONExpect(ctx, issuerBaseURL+"/test/queue-user", body, http.StatusNoContent)
}

// postJSONExpect POSTs body as application/json and returns an error
// (including the response body) unless the response status is exactly
// wantStatus.
func postJSONExpect(ctx context.Context, url string, body []byte, wantStatus int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building request to %s: %w", url, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != wantStatus {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, respBody)
	}

	return nil
}

// MintToken drives mockoidc's real authorization-code + token-exchange HTTP
// flow (the same one auth.NewVerifier's discovery-based verifier expects to
// validate) and returns a real, signed ID token. clientID/clientSecret come
// from mockoidc's logged Config() at startup (see
// test/functional/support/mockoidc/main.go) and must match whatever
// OIDC_AUDIENCE the app under test was configured with.
func MintToken(ctx context.Context, issuerURL, clientID, clientSecret string) (string, error) {
	httpClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	authorizeQuery := url.Values{}
	authorizeQuery.Set("client_id", clientID)
	authorizeQuery.Set("scope", "openid email profile")
	authorizeQuery.Set("response_type", "code")
	authorizeQuery.Set("redirect_uri", "http://127.0.0.1/callback")
	authorizeQuery.Set("state", "func-test-state")
	authorizeQuery.Set("nonce", "func-test-nonce")

	authorizeURL := issuerURL + "/authorize?" + authorizeQuery.Encode()

	authorizeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, authorizeURL, nil)
	if err != nil {
		return "", fmt.Errorf("building authorize request: %w", err)
	}

	resp, err := httpClient.Do(authorizeReq)
	if err != nil {
		return "", fmt.Errorf("calling authorize endpoint: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	location, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("reading authorize redirect: %w", err)
	}
	code := location.Query().Get("code")
	if code == "" {
		return "", fmt.Errorf("authorize response missing code")
	}

	tokenForm := url.Values{}
	tokenForm.Set("client_id", clientID)
	tokenForm.Set("client_secret", clientSecret)
	tokenForm.Set("grant_type", "authorization_code")
	tokenForm.Set("code", code)

	tokenReq, err := http.NewRequestWithContext(ctx, http.MethodPost, issuerURL+"/token", strings.NewReader(tokenForm.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token request: %w", err)
	}
	tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	tokenResp, err := httpClient.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("calling token endpoint: %w", err)
	}
	defer func() { _ = tokenResp.Body.Close() }()

	body, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token response: %w", err)
	}

	var tokens struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tokens); err != nil {
		return "", fmt.Errorf("decoding token response: %w", err)
	}
	if tokens.IDToken == "" {
		return "", fmt.Errorf("token response missing id_token: %s", body)
	}

	return tokens.IDToken, nil
}

// TokenSubject returns the "sub" claim of an ID token, decoded without
// signature verification. Fine for functional tests: the caller already
// obtained this token from a trusted issuer (mockoidc or, in CI, real
// Cognito) via AuthToken/AdminAuthToken/SecondUserAuthToken - this just
// reads its subject back out. Needed because CI's real-Cognito-token path
// mints a real Cognito-generated subject, not the fixed
// fixtures.TestUserSubject-style constants mockoidc uses - specs that need
// to know "my own subject" (e.g. to seed owned fixture data) can't assume a
// fixed value across both environments.
func TokenSubject(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed ID token: expected 3 dot-separated parts, got %d", len(parts))
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

// authToken mints a token for the given fixture identity via mockoidc,
// unless envOverride is set (a real Cognito-minted token, when
// support.BaseURL() points at a real deployed stack instead) - a local
// mockoidc instance and real Cognito aren't interchangeable token issuers,
// so this can't be derived from support.BaseURL() alone.
func authToken(ctx context.Context, envOverride, subject string, groups []string) (string, error) {
	if v := os.Getenv(envOverride); v != "" {
		return v, nil
	}
	if err := QueueUser(ctx, support.MockOIDCBaseURL(), subject, groups); err != nil {
		return "", err
	}
	return MintToken(ctx, support.MockOIDCTokenURL(), support.MockOIDCClientID, support.MockOIDCClientSecret)
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

// SecondUserAuthToken returns a valid bearer token for a second, unrelated
// plain (non-admin) test user - distinct from AuthToken's identity, for
// exercising ownership/visibility-scoped reads of another user's items. See
// authToken for the KBDB_SECOND_USER_AUTH_TOKEN override behavior.
func SecondUserAuthToken(ctx context.Context) (string, error) {
	return authToken(ctx, "KBDB_SECOND_USER_AUTH_TOKEN", fixtures.SecondUserSubject, nil)
}
