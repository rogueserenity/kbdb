// Package api holds HTTP clients for the functional test suites: a generic
// authenticated-request Client, plus typed clients per REST entity/MCP on
// top of it. See ../db for the DynamoDB seed/cleanup counterpart.
package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// Client issues authenticated requests against support.BaseURL(). Zero
// value is usable directly (equivalent to &Client{}).
type Client struct{}

// NewClient returns a ready-to-use Client.
func NewClient() *Client {
	return &Client{}
}

// Do sends method to path (relative to support.BaseURL(), e.g.
// "/users/alice/switches") with the given bearer token (omit auth entirely
// if token is empty - not every route requires one) and body (nil for no
// body). The returned response's Body is fully buffered in memory and
// already closed - not a live network read - so it's safe to decode from a
// Ginkgo node other than the one that called Do. That's necessary, not
// just convenient: Ginkgo cancels each node's SpecContext the instant that
// node's own body function returns (see internal/suite.go's
// `defer sc.cancel(...)` in Ginkgo's runNode), and a BeforeEach's ctx is
// exactly the ctx wired into the *http.Request here via
// http.NewRequestWithContext - so a live resp.Body read from a later It
// node intermittently failed with "spec has finished" once the BeforeEach
// that issued the request had already returned and its context was
// cancelled, even though the It itself has its own, still-live context.
func (c *Client) Do(ctx context.Context, method, path, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, support.BaseURL()+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(buf))

	return resp, nil
}
