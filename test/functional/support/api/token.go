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
// doesn't already exist. A 409 (already exists) is not an error - specs may
// call this repeatedly across a suite run, and the fixture users are also
// pre-seeded via scripts/workos-emulate-seed.yaml, so this call is expected
// to hit the "already exists" case on every fresh emulator startup.
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
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
