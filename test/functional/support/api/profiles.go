package api

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// ListProfilesQuery is the optional query for ProfilesClient.List. A zero
// value lists the whole directory with the server's default limit.
type ListProfilesQuery struct {
	// Limit, if >= 0, is sent as the limit query param; a negative value
	// omits it, letting the server apply its default.
	Limit           int
	Cursor          string
	Username        string
	DiscordUsername string
}

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

// List calls GET /v1/profiles with the given bearer token (empty for an
// anonymous request). The caller owns closing resp.Body.
func (c *ProfilesClient) List(ctx context.Context, token string, q ListProfilesQuery) (*http.Response, error) {
	query := url.Values{}
	if q.Limit >= 0 {
		query.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Cursor != "" {
		query.Set("cursor", q.Cursor)
	}
	if q.Username != "" {
		query.Set("username", q.Username)
	}
	if q.DiscordUsername != "" {
		query.Set("discord_username", q.DiscordUsername)
	}

	path := "/v1/profiles"
	if encoded := query.Encode(); encoded != "" {
		path += "?" + encoded
	}

	return c.client.Do(ctx, http.MethodGet, path, token, nil)
}

// Create calls POST /v1/profile/{userId}. The caller owns closing resp.Body.
func (c *ProfilesClient) Create(ctx context.Context, userID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPost, "/v1/profile/"+userID, token, bytes.NewBufferString(body))
}

// Update calls PUT /v1/profile/{userId}. The caller owns closing resp.Body.
func (c *ProfilesClient) Update(ctx context.Context, userID, token, body string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodPut, "/v1/profile/"+userID, token, bytes.NewBufferString(body))
}

// Delete calls DELETE /v1/profile/{userId}. The caller owns closing
// resp.Body.
func (c *ProfilesClient) Delete(ctx context.Context, userID, token string) (*http.Response, error) {
	return c.client.Do(ctx, http.MethodDelete, "/v1/profile/"+userID, token, nil)
}
