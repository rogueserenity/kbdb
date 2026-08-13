package api

import (
	"bytes"
	"context"
	"net/http"
)

// BuildsClient calls the /v1/users/{userId}/builds routes. Built on Client
// so specs don't hand-build requests/auth headers per route.
type BuildsClient struct {
	client *Client
}

// NewBuildsClient returns a ready-to-use BuildsClient.
func NewBuildsClient() *BuildsClient {
	return &BuildsClient{client: NewClient()}
}

// Create calls POST /v1/users/{ownerID}/builds with body as the raw JSON
// request body and the given bearer token (empty to omit the Authorization
// header entirely, e.g. for an unauthenticated-request spec). The caller
// owns closing resp.Body.
func (c *BuildsClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/builds", token, bytes.NewBufferString(body))
}

// Get calls GET /v1/users/{ownerID}/builds/{buildId} with the given bearer
// token (empty for an anonymous request). The caller owns closing
// resp.Body.
func (c *BuildsClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/users/"+ownerID+"/builds/"+id, token, nil)
}
