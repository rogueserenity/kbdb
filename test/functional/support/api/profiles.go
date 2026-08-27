package api

import (
	"bytes"
	"context"
	"net/http"
)

// ProfilesClient calls the /v1/profile and /v1/profiles routes.
type ProfilesClient struct {
	client *Client
}

// NewProfilesClient returns a ready-to-use ProfilesClient.
func NewProfilesClient() *ProfilesClient {
	return &ProfilesClient{client: NewClient()}
}

// Get calls GET /v1/profile/{identifier} (token empty for anonymous). The
// caller owns closing resp.Body.
func (c *ProfilesClient) Get(ctx context.Context, identifier, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/profile/"+identifier, token, nil)
}

// Create calls POST /v1/profile/{userId}. The caller owns closing resp.Body.
func (c *ProfilesClient) Create(ctx context.Context, userID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/profile/"+userID, token, bytes.NewBufferString(body))
}

// Update calls PUT /v1/profile/{userId}. The caller owns closing resp.Body.
func (c *ProfilesClient) Update(ctx context.Context, userID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/profile/"+userID, token, bytes.NewBufferString(body))
}
