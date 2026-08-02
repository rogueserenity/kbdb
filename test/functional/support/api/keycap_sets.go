package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// KeycapSetsClient calls the /v1/users/{userId}/keycap-sets routes. Built
// on Client so specs don't hand-build requests/auth headers per route.
type KeycapSetsClient struct {
	client *Client
}

// NewKeycapSetsClient returns a ready-to-use KeycapSetsClient.
func NewKeycapSetsClient() *KeycapSetsClient {
	return &KeycapSetsClient{client: NewClient()}
}

// List calls GET /v1/users/{ownerID}/keycap-sets with the given bearer
// token (empty for an anonymous request). limit, if >= 0, is sent as the
// limit query parameter; a negative limit omits it entirely, letting the
// server apply its default. The caller owns closing resp.Body.
func (c *KeycapSetsClient) List(ctx context.Context, ownerID, token string, limit int) (*http.Response, error) {
	path := "/v1/users/" + ownerID + "/keycap-sets"
	if limit >= 0 {
		query := url.Values{"limit": []string{strconv.Itoa(limit)}}
		path += "?" + query.Encode()
	}

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}

// ListWithRawLimit is List, but sends limit verbatim rather than requiring
// an int - lets a spec send a non-numeric limit to check the OpenAPI
// validator's own rejection, which List's typed parameter can't express.
// The caller owns closing resp.Body.
func (c *KeycapSetsClient) ListWithRawLimit(ctx context.Context, ownerID, token, limit string) (*http.Response, error) {
	query := url.Values{"limit": []string{limit}}
	path := "/v1/users/" + ownerID + "/keycap-sets?" + query.Encode()

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}

// Get calls GET /v1/users/{ownerID}/keycap-sets/{id} with the given bearer
// token (empty for an anonymous request). The caller owns closing
// resp.Body.
func (c *KeycapSetsClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/users/"+ownerID+"/keycap-sets/"+id, token, nil)
}

// Create calls POST /v1/users/{ownerID}/keycap-sets with body as the raw
// JSON request body and the given bearer token (empty for an anonymous
// request). The caller owns closing resp.Body.
func (c *KeycapSetsClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/keycap-sets", token, bytes.NewBufferString(body))
}

// Update calls PUT /v1/users/{ownerID}/keycap-sets/{id} with body as the
// raw JSON request body and the given bearer token (empty for an
// anonymous request). The caller owns closing resp.Body.
func (c *KeycapSetsClient) Update(ctx context.Context, ownerID, id, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/users/"+ownerID+"/keycap-sets/"+id, token, bytes.NewBufferString(body))
}

// Delete calls DELETE /v1/users/{ownerID}/keycap-sets/{id} with the given
// bearer token (empty for an anonymous request). The caller owns closing
// resp.Body.
func (c *KeycapSetsClient) Delete(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodDelete, "/v1/users/"+ownerID+"/keycap-sets/"+id, token, nil)
}

// CreateKit calls POST /v1/users/{ownerID}/keycap-sets/{setID}/kits with
// body as the raw JSON request body and the given bearer token (empty for
// an anonymous request). The caller owns closing resp.Body.
func (c *KeycapSetsClient) CreateKit(ctx context.Context, ownerID, setID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/keycap-sets/"+setID+"/kits", token, bytes.NewBufferString(body))
}
