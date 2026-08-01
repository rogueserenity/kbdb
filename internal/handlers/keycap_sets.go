package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ListKeycapSets reads the {userId} path value and lists that owner's
// keycap sets. Anonymous callers are allowed; visibility is scoped to what
// the caller (if any) may read, per internal/authz.
func ListKeycapSets(repo repository.KeycapSetRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		sets, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing keycap sets", "error", err)
			problem.Internal(w, "failed to list keycap sets")
			return
		}

		items := make([]api.KeycapSetSummary, len(sets))
		for i, ks := range sets {
			items[i] = repoapi.KeycapSetToAPISummary(ks)
		}

		page := api.KeycapSetListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}
