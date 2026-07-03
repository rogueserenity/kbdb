package middleware

import (
	"net/http"
	"strings"

	"github.com/rogueserenity/kbdb/internal/auth"
)

// Auth verifies the bearer token on every request independently of any
// upstream API Gateway authorizer (defense-in-depth — see project plan). On
// success it attaches the verified user ID to the request-scoped logger.
func Auth(verifier *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := BearerToken(r)
			if !ok {
				http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
				return
			}

			claims, err := verifier.VerifyToken(r.Context(), rawToken)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := WithUserID(r.Context(), claims.Subject)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// BearerToken extracts the raw bearer token from r's Authorization header.
// Shared by the REST auth middleware and the MCP server's HTTP context
// function, since both need to pull the same token out of the same header.
func BearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return strings.TrimPrefix(h, prefix), true
}
