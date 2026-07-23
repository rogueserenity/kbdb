package middleware

import (
	"net/http"
	"slices"

	"github.com/rogueserenity/kbdb/internal/auth"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/problem"
)

// RequireAdmin rejects the request with 403 unless the caller's
// cognito:groups (see internal/ctx.Groups, populated by Auth) includes
// auth.AdminsGroup. Must run after Auth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !slices.Contains(ctxpkg.Groups(r.Context()), auth.AdminsGroup) {
			problem.Forbidden(w, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
