package support

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// QueueUser tells the mockoidc instance at issuerBaseURL (its host:port,
// not including the /oidc path prefix MintToken's issuerURL uses) which
// fixture identity the next /oidc/authorize call should authenticate as.
// mockoidc.UserQueue is a strict FIFO with no way to select a specific
// queued user, and this process can't call mockoidc.MockOIDC.QueueUser
// directly since the server runs in a separate process/container - this
// drives the same effect over its /test/queue-user control endpoint (see
// test/functional/support/mockoidc/main.go). groups may be nil/empty for
// a plain (non-admin) user.
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
