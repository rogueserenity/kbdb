package middleware

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/auth"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	logpkg "github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
)

// OptionalAuth verifies the bearer token if one is present, for routes that
// allow anonymous callers (e.g. reads on items whose visibility may permit
// public or authenticated-but-not-owner access — see
// [github.com/rogueserenity/kbdb/internal/authz]). A missing token is not
// an error and the request proceeds with no user ID on its context; a
// present-but-invalid token is still rejected with 401, since silently
// treating it as anonymous would hide a real client error.
//
// Required-auth routes no longer have an equivalent in-process middleware:
// they rely solely on API Gateway's native JWT authorizer (see
// template.yaml's HttpApi.Auth.DefaultAuthorizer), which verifies the
// identical token before the request ever reaches this process. That
// in-process re-verification was deliberate defense-in-depth under
// Cognito; re-examined for the WorkOS migration and dropped as
// redundant — the native authorizer is the same class of AWS-verified
// mechanism Cognito's JWT authorizer already was.
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
				problem.Unauthorized(w, "invalid token")
				return
			}

			next.ServeHTTP(w, authedReq)
		})
	}
}

// RequireAuthorizerIdentity reads the caller's identity from the
// X-Amzn-Request-Context header API Gateway's native JWT authorizer
// populates via aws-lambda-web-adapter (see authorizer_context.go) and
// writes it into ctxpkg/logpkg, the same target OptionalAuth's
// authenticate() writes to. Unlike OptionalAuth, a missing/unparseable
// identity here IS an error (500, not 401) - reaching this middleware at
// all means the route's Auth block has no NONE override, so API Gateway
// already rejected any request without a valid token before it got here;
// if this middleware still finds no identity, that's a misconfiguration
// (e.g. this route wired to the wrong authorizer, or running outside API
// Gateway/aws-lambda-web-adapter entirely), not a legitimate
// unauthenticated request to reject with 401.
func RequireAuthorizerIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sub, ok := authorizerSubject(r)
		if !ok {
			logpkg.FromContext(r.Context()).Error("required-auth route reached with no verified identity from API Gateway authorizer")
			problem.Internal(w, "server misconfiguration")
			return
		}

		ctx := ctxpkg.WithUserID(r.Context(), sub)
		l := logpkg.WithUserID(logpkg.FromContext(ctx), sub)
		ctx = logpkg.WithLogger(ctx, l)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticate verifies rawToken and returns r with its context updated to
// carry the verified caller's user ID, groups, and request-scoped logger.
func authenticate(r *http.Request, verifier *auth.Verifier, rawToken string) (*http.Request, error) {
	claims, err := verifier.VerifyToken(r.Context(), rawToken)
	if err != nil {
		// Warn, not Error: an individual invalid/expired token from one
		// client is expected traffic, not a bug - still worth a trace to
		// spot a misconfigured client or repeated probing.
		logpkg.FromContext(r.Context()).Warn("token verification failed", logpkg.Error, err)
		return nil, err
	}

	ctx := ctxpkg.WithUserID(r.Context(), claims.Subject)
	ctx = ctxpkg.WithGroups(ctx, claims.Groups)
	l := logpkg.WithUserID(logpkg.FromContext(ctx), claims.Subject)
	ctx = logpkg.WithLogger(ctx, l)

	return r.WithContext(ctx), nil
}
