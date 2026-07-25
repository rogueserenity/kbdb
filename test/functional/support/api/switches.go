package api

import (
	"bytes"
	"context"
	"net/http"
)

// SwitchesClient calls the /users/{userId}/switches routes. Built on
// Client so specs don't hand-build requests/auth headers per route.
type SwitchesClient struct {
	client *Client
}

// NewSwitchesClient returns a ready-to-use SwitchesClient.
func NewSwitchesClient() *SwitchesClient {
	return &SwitchesClient{client: NewClient()}
}

// List calls GET /users/{ownerID}/switches with the given bearer token
// (empty for an anonymous request). The caller owns closing resp.Body.
func (c *SwitchesClient) List(ctx context.Context, ownerID, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/users/"+ownerID+"/switches", token, nil)
}

// Get calls GET /users/{ownerID}/switches/{id} with the given bearer token
// (empty for an anonymous request). The caller owns closing resp.Body.
func (c *SwitchesClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/users/"+ownerID+"/switches/"+id, token, nil)
}

// Create calls POST /users/{ownerID}/switches with body as the raw JSON
// request body and the given bearer token (empty to omit the Authorization
// header entirely, e.g. for an unauthenticated-request spec). The caller
// owns closing resp.Body.
func (c *SwitchesClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/users/"+ownerID+"/switches", token, bytes.NewBufferString(body))
}
