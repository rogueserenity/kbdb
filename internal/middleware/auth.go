package middleware

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/auth"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	logpkg "github.com/rogueserenity/kbdb/internal/log"
)

// Auth verifies the bearer token on every request independently of any
// upstream API Gateway authorizer (defense-in-depth — see project plan). On
// success it attaches the verified user ID to the request context and the
// request-scoped logger.
func Auth(verifier *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := auth.BearerToken(r)
			if !ok {
				http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
				return
			}

			claims, err := verifier.VerifyToken(r.Context(), rawToken)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := ctxpkg.WithUserID(r.Context(), claims.Subject)
			ctx = ctxpkg.WithGroups(ctx, claims.Groups)
			l := logpkg.WithUserID(logpkg.FromContext(ctx), claims.Subject)
			ctx = logpkg.WithLogger(ctx, l)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
