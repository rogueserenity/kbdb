package api

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// KeyboardsClient calls the /v1/users/{userId}/keyboards routes. Built on
// Client so specs don't hand-build requests/auth headers per route.
type KeyboardsClient struct {
	client *Client
}

// NewKeyboardsClient returns a ready-to-use KeyboardsClient.
func NewKeyboardsClient() *KeyboardsClient {
	return &KeyboardsClient{client: NewClient()}
}

// List calls GET /v1/users/{ownerID}/keyboards with the given bearer token
// (empty for an anonymous request). limit, if >= 0, is sent as the limit
// query parameter; a negative limit omits it entirely, letting the server
// apply its default. The caller owns closing resp.Body.
func (c *KeyboardsClient) List(ctx context.Context, ownerID, token string, limit int) (*http.Response, error) {
	path := "/v1/users/" + ownerID + "/keyboards"
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
func (c *KeyboardsClient) ListWithRawLimit(ctx context.Context, ownerID, token, limit string) (*http.Response, error) {
	query := url.Values{"limit": []string{limit}}
	path := "/v1/users/" + ownerID + "/keyboards?" + query.Encode()

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}
