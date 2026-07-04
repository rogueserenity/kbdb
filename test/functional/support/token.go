package support

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// MintToken drives mockoidc's real authorization-code + token-exchange HTTP
// flow (the same one auth.NewVerifier's discovery-based verifier expects to
// validate) and returns a real, signed ID token. clientID/clientSecret come
// from mockoidc's logged Config() at startup (see
// test/functional/support/mockoidc/main.go) and must match whatever
// OIDC_AUDIENCE the app under test was configured with.
func MintToken(issuerURL, clientID, clientSecret string) (string, error) {
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

	authorizeReq, err := http.NewRequest(http.MethodGet, authorizeURL, nil)
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

	tokenReq, err := http.NewRequest(http.MethodPost, issuerURL+"/token", strings.NewReader(tokenForm.Encode()))
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
