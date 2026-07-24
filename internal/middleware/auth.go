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
// request-scoped logger. A missing or invalid token is always rejected with
// 401 — use OptionalAuth for routes that allow anonymous callers.
func Auth(verifier *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := auth.BearerToken(r)
			if !ok {
				http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
				return
			}

			authedReq, err := authenticate(r, verifier, rawToken)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, authedReq)
		})
	}
}

// OptionalAuth verifies the bearer token if one is present, for routes that
// allow anonymous callers (e.g. reads on items whose visibility may permit
// public or authenticated-but-not-owner access — see internal/authz). A
// missing token is not an error and the request proceeds with no user ID on
// its context; a present-but-invalid token is still rejected with 401,
// since silently treating it as anonymous would hide a real client error.
func OptionalAuth(verifier *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := auth.BearerToken(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			authedReq, err := authenticate(r, verifier, rawToken)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, authedReq)
		})
	}
}

// authenticate verifies rawToken and returns r with its context updated to
// carry the verified caller's user ID, groups, and request-scoped logger.
func authenticate(r *http.Request, verifier *auth.Verifier, rawToken string) (*http.Request, error) {
	claims, err := verifier.VerifyToken(r.Context(), rawToken)
	if err != nil {
		return nil, err
	}

	ctx := ctxpkg.WithUserID(r.Context(), claims.Subject)
	ctx = ctxpkg.WithGroups(ctx, claims.Groups)
	l := logpkg.WithUserID(logpkg.FromContext(ctx), claims.Subject)
	ctx = logpkg.WithLogger(ctx, l)

	return r.WithContext(ctx), nil
}
