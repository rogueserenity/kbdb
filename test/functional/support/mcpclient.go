package support

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// MCPClient is a minimal JSON-RPC client for the MCP Streamable HTTP
// transport, driving a real running server (not a mock) the same way
// mark3labs/mcp-go's own transport works - see internal/mcp. It exists
// because Streamable HTTP requires session continuity (an initialize call
// returns an Mcp-Session-Id header that must be replayed on subsequent
// calls, or the server responds "Invalid session ID"), which a bare
// http.Client can't do for you.
type MCPClient struct {
	baseURL   string
	authToken string
	sessionID string
}

// NewMCPClient returns a client targeting baseURL (e.g. "http://host/mcp").
// authToken, if non-empty, is sent as a Bearer token on every request.
func NewMCPClient(baseURL, authToken string) *MCPClient {
	return &MCPClient{baseURL: baseURL, authToken: authToken}
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

// RPCResponse is a decoded MCP/JSON-RPC response envelope.
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   json.RawMessage `json:"error,omitempty"`
}

// Initialize performs the MCP initialize handshake and captures the
// resulting session ID for use on subsequent calls.
func (c *MCPClient) Initialize() (*RPCResponse, error) {
	return c.call(1, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "kbdb-functional-test", "version": "1.0"},
	})
}

// CallTool performs a tools/call request for the named tool.
func (c *MCPClient) CallTool(name string, arguments map[string]any) (*RPCResponse, error) {
	return c.call(2, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
}

func (c *MCPClient) call(id int, method string, params any) (*RPCResponse, error) {
	body, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return nil, fmt.Errorf("encoding request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP %d: %s", method, resp.StatusCode, respBody)
	}

	var rpcResp RPCResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w: %s", method, err, respBody)
	}
	return &rpcResp, nil
}
