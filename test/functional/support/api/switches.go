package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SwitchesClient calls the /v1/users/{userId}/switches routes. Built on
// Client so specs don't hand-build requests/auth headers per route.
type SwitchesClient struct {
	client *Client
}

// NewSwitchesClient returns a ready-to-use SwitchesClient.
func NewSwitchesClient() *SwitchesClient {
	return &SwitchesClient{client: NewClient()}
}

// List calls GET /v1/users/{ownerID}/switches with the given bearer token
// (empty for an anonymous request). limit, if >= 0, is sent as the limit
// query parameter; a negative limit omits it entirely, letting the server
// apply its default. The caller owns closing resp.Body.
func (c *SwitchesClient) List(ctx context.Context, ownerID, token string, limit int) (*http.Response, error) {
	path := "/v1/users/" + ownerID + "/switches"
	if limit >= 0 {
		query := url.Values{"limit": []string{strconv.Itoa(limit)}}
		path += "?" + query.Encode()
	}

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}

// Get calls GET /v1/users/{ownerID}/switches/{id} with the given bearer
// token (empty for an anonymous request). The caller owns closing resp.Body.
func (c *SwitchesClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/users/"+ownerID+"/switches/"+id, token, nil)
}

// Create calls POST /v1/users/{ownerID}/switches with body as the raw JSON
// request body and the given bearer token (empty to omit the Authorization
// header entirely, e.g. for an unauthenticated-request spec). The caller
// owns closing resp.Body.
func (c *SwitchesClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/switches", token, bytes.NewBufferString(body))
}
