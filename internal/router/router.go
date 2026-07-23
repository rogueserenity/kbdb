// Package router builds the application's HTTP routes.
package router

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/handlers"
	"github.com/rogueserenity/kbdb/internal/mcp"
	"github.com/rogueserenity/kbdb/internal/middleware"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// New builds the application's http.Handler. verifier authenticates every
// request; additional entities/routes are added here in later issues, on
// this same handler.
//
// issuerURL configures the MCP endpoint's RFC 9728 Protected Resource
// Metadata (the OIDC issuer MCP clients should authenticate against); the
// metadata's "resource" field is derived per-request rather than passed in
// statically — see internal/mcp.Handlers doc comment for why. version is
// advertised to MCP clients in the server's initialize handshake.
func New(verifier *auth.Verifier, lookupRepo repository.LookupRepository, issuerURL, version string) http.Handler {
	mux := http.NewServeMux()

	// REST: auth failures are HTTP-level (401) — appropriate for REST.
	mux.Handle("GET /v1/ping", middleware.Auth(verifier)(http.HandlerFunc(handlers.Ping)))

	// No middleware.Auth: security: [] in api/openapi.yaml.
	mux.Handle("GET /v1/lookups", handlers.ListLookups(lookupRepo))
	mux.Handle("GET /v1/lookups/{category}", handlers.GetLookup(lookupRepo))
	mux.Handle("POST /v1/lookups/{category}",
		middleware.Auth(verifier)(middleware.RequireAdmin(handlers.CreateLookup(lookupRepo))))

	// MCP: auth happens inside the MCP server itself (context func + tool
	// handler middleware), returning MCP-shaped errors rather than a bare
	// HTTP 401, since MCP clients expect protocol-level errors for a failed
	// tool call. middleware.Auth is deliberately NOT applied here.
	mcpHandlers := mcp.New(verifier, issuerURL, version)
	mux.Handle("/mcp", mcpHandlers.Streamable)
	mux.Handle(mcpHandlers.MetadataPath, mcpHandlers.Metadata)

	return middleware.Logging(mux)
}
