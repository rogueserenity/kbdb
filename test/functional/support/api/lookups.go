package api

import (
	"bytes"
	"context"
	"net/http"
)

// LookupsClient calls the /v1/lookups routes. Built on Client so specs
// don't hand-build requests/auth headers per route.
type LookupsClient struct {
	client *Client
}

// NewLookupsClient returns a ready-to-use LookupsClient.
func NewLookupsClient() *LookupsClient {
	return &LookupsClient{client: NewClient()}
}

// ListCategories calls GET /v1/lookups. The caller owns closing resp.Body.
func (c *LookupsClient) ListCategories(ctx context.Context) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/lookups", "", nil)
}

// GetCategory calls GET /v1/lookups/{category}. The caller owns closing
// resp.Body.
func (c *LookupsClient) GetCategory(ctx context.Context, category string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodGet, "/v1/lookups/"+category, "", nil)
}

// CreateCategory calls POST /v1/lookups/{category} with body as the raw
// JSON request body and the given bearer token (empty to omit the
// Authorization header entirely). The caller owns closing resp.Body.
func (c *LookupsClient) CreateCategory(ctx context.Context, category, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/lookups/"+category, token, bytes.NewBufferString(body))
}

// ReplaceCategory calls PUT /v1/lookups/{category} with body as the raw
// JSON request body and the given bearer token (empty to omit the
// Authorization header entirely). The caller owns closing resp.Body.
func (c *LookupsClient) ReplaceCategory(ctx context.Context, category, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/lookups/"+category, token, bytes.NewBufferString(body))
}

// DeleteCategory calls DELETE /v1/lookups/{category} with the given bearer
// token (empty to omit the Authorization header entirely). The caller owns
// closing resp.Body.
func (c *LookupsClient) DeleteCategory(ctx context.Context, category, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodDelete, "/v1/lookups/"+category, token, nil)
}
