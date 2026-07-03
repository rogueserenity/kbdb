// Package router builds the application's HTTP routes.
package router

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/auth"
	"github.com/rogueserenity/kbdb/internal/handlers"
	"github.com/rogueserenity/kbdb/internal/middleware"
)

// New builds the application's http.Handler. verifier authenticates every
// request; additional entities/routes and the MCP endpoint are added here in
// later issues, on this same handler.
func New(verifier *auth.Verifier) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/ping", handlers.Ping)

	var handler http.Handler = mux
	handler = middleware.Auth(verifier)(handler)
	handler = middleware.Logging(handler)
	return handler
}
