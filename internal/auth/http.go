package auth

import (
	"net/http"
	"strings"
)

// BearerToken extracts the raw bearer token from r's Authorization header.
// Shared by the REST auth middleware (internal/middleware) and the MCP
// server's HTTP context function (internal/mcp), since both are HTTP-based
// transports that need to pull the same token out of the same header.
//
// [Verifier]/[Verifier.VerifyToken] remain deliberately protocol-agnostic;
// this helper is the one place internal/auth takes on a net/http dependency,
// since extracting a bearer token from an HTTP request is inherently
// HTTP-shaped regardless of which higher-level protocol (REST, MCP) is
// carried over it.
func BearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimPrefix(h, prefix), true
}
