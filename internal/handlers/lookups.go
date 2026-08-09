package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
)

// ListLookups reads no path values and requires no auth. Returns category
// names only, not their values (see GetLookup for that).
func ListLookups(w http.ResponseWriter, r *http.Request) {
	names := lookup.ListCategoryNames(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(names)
}

// writeLookup writes l as the response body, or a 500 if its stored values
// don't match the shape its category expects.
func writeLookup(ctx context.Context, w http.ResponseWriter, status int, l lookup.Lookup) {
	out, err := repoapi.LookupToAPI(l)
	if err != nil {
		log.FromContext(ctx).Error("mapping lookup category to API shape", log.LookupCategory, string(l.Category), log.Error, err)
		problem.Internal(w, "failed to read lookup category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

// GetLookup reads the {category} path value and requires no auth.
func GetLookup(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")

	l, ok := lookup.GetCategory(r.Context(), lookup.Category(category))
	if !ok {
		problem.NotFound(w, "resource not found")
		return
	}

	writeLookup(r.Context(), w, http.StatusOK, l)
}
