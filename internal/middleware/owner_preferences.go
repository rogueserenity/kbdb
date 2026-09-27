package middleware

import (
	"net/http"

	"github.com/rogueserenity/kbdb/internal/ownerprefs"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// OwnerPreferences puts a loader for the {userId} path segment's owner
// preferences on the request context, for [ownerprefs.Get]. It must wrap a
// route whose pattern includes {userId}.
func OwnerPreferences(reader repository.PreferencesReader) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := ownerprefs.WithLoader(r.Context(), reader, r.PathValue("userId"))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
