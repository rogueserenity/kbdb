package api

import (
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

// Get calls GET /v1/profile/{identifier} with the given bearer token (empty
// for an anonymous request). identifier is a user id or a username. The
// caller owns closing resp.Body.
func (c *ProfilesClient) Get(ctx context.Context, identifier, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/profile/"+identifier, token, nil)
}
