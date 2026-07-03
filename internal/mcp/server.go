// Package mcp wires the mcp-go MCP server into the application, reusing the
// same auth.Verifier as REST rather than a second verification
// implementation — see internal/auth for the shared verification logic and
// internal/middleware for the REST-side adapter.
package mcp

import (
	"context"
	"encoding/json"
	"net/http"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/middleware"
)

type contextKey int

const bearerTokenContextKey contextKey = iota

// Handlers holds the two HTTP handlers the router needs to mount: the MCP
// Streamable HTTP endpoint itself, and the RFC 9728 Protected Resource
// Metadata handler. Go's net/http.ServeMux won't route requests for the
// well-known metadata path to the streamable handler just because it's
// registered at "/mcp" (an exact-match mux pattern shadows it) — both must
// be mounted explicitly.
//
// The metadata handler builds its "resource" URL from each request's Host
// header rather than a static, deploy-time-known value. This isn't just a
// workaround: referencing the API Gateway's own generated domain from a
// CloudFormation env var on this same Lambda creates a genuine circular
// dependency (the function's Events block makes the API depend on the
// function; a !Sub referencing the API's logical ID would make the function
// depend back on the API). Deriving it from the request is also more
// correct for RFC 9728 — the "resource" identifier should reflect whatever
// URL the client actually used to reach the server, which stays right even
// if a custom domain is added later.
type Handlers struct {
	Streamable   http.Handler
	MetadataPath string
	Metadata     http.Handler
}

// New builds the MCP server. issuerURL is the OIDC issuer MCP clients should
// authenticate against, advertised via RFC 9728 Protected Resource Metadata.
// version is advertised to MCP clients in the initialize handshake.
func New(verifier *auth.Verifier, issuerURL, version string) Handlers {
	mcpServer := server.NewMCPServer("kbdb", version,
		server.WithToolCapabilities(true),
		server.WithToolHandlerMiddleware(authMiddleware(verifier)),
	)

	registerTools(mcpServer)

	streamable := server.NewStreamableHTTPServer(mcpServer,
		server.WithHTTPContextFunc(func(ctx context.Context, r *http.Request) context.Context {
			token, _ := middleware.BearerToken(r)
			return context.WithValue(ctx, bearerTokenContextKey, token)
		}),
	)

	return Handlers{
		Streamable:   streamable,
		MetadataPath: server.WellKnownProtectedResourcePath,
		Metadata:     metadataHandler(issuerURL),
	}
}

// metadataHandler serves RFC 9728 Protected Resource Metadata, building the
// "resource" field from the request's Host header (see Handlers doc comment
// for why this is derived per-request rather than statically configured).
func metadataHandler(issuerURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		scheme := "https"
		if fp := r.Header.Get("X-Forwarded-Proto"); fp != "" {
			scheme = fp
		}
		resourceURL := scheme + "://" + r.Host + "/mcp"

		config := server.ProtectedResourceMetadataConfig{
			Resource:             resourceURL,
			AuthorizationServers: []string{issuerURL},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(config)
	})
}

// authMiddleware verifies the bearer token stashed into context by New's
// HTTPContextFunc, using the same auth.Verifier as REST. Failures return an
// MCP-shaped tool error (not an HTTP status) — MCP clients expect
// protocol-level errors for a failed tool call, not a bare HTTP failure.
func authMiddleware(verifier *auth.Verifier) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
			token, _ := ctx.Value(bearerTokenContextKey).(string)
			if token == "" {
				return gomcp.NewToolResultError("missing or malformed authorization header"), nil
			}

			claims, err := verifier.VerifyToken(ctx, token)
			if err != nil {
				return gomcp.NewToolResultError("invalid token"), nil
			}

			ctx = middleware.WithUserID(ctx, claims.Subject)
			return next(ctx, req)
		}
	}
}
