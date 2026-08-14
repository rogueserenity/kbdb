package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
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

// Create calls POST /v1/users/{ownerID}/builds.
func (c *BuildsClient) Create(ctx context.Context, ownerID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/users/"+ownerID+"/builds", token, bytes.NewBufferString(body))
}

// Get calls GET /v1/users/{ownerID}/builds/{id}.
func (c *BuildsClient) Get(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/users/"+ownerID+"/builds/"+id, token, nil)
}

// Update calls PUT /v1/users/{ownerID}/builds/{id}.
func (c *BuildsClient) Update(ctx context.Context, ownerID, id, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/users/"+ownerID+"/builds/"+id, token, bytes.NewBufferString(body))
}

// Delete calls DELETE /v1/users/{ownerID}/builds/{id}.
func (c *BuildsClient) Delete(ctx context.Context, ownerID, id, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodDelete, "/v1/users/"+ownerID+"/builds/"+id, token, nil)
}

// List sends limit as the query parameter when >= 0; a negative limit
// omits it, letting the server apply its default.
func (c *BuildsClient) List(ctx context.Context, ownerID, token string, limit int) (*http.Response, error) {
	path := "/v1/users/" + ownerID + "/builds"
	if limit >= 0 {
		query := url.Values{"limit": []string{strconv.Itoa(limit)}}
		path += "?" + query.Encode()
	}

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}
