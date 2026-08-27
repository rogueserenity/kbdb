package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/profileread"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// GetProfile reads the {identifier} path value - an IdP subject or a
// username - and returns that profile. Anonymous callers are allowed. A
// non-discoverable profile is returned only to its owner; to anyone else,
// and for an identifier matching no profile, the response is 404 (not 403)
// so a non-discoverable profile's existence isn't revealed.
func GetProfile(repo repository.ProfileRepository, images repository.ProfileImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identifier := r.PathValue("identifier")

		p, ok, err := profileread.Resolve(r.Context(), repo, identifier)
		if err != nil {
			log.FromContext(r.Context()).Error("getting profile", log.Error, err, log.ProfileID, identifier)
			problem.Internal(w, "failed to get profile")
			return
		}
		if !ok {
			problem.NotFound(w, "resource not found")
			return
		}

		out, err := repoapi.ProfileToAPI(r.Context(), *p, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping profile to API", log.Error, err, log.ProfileID, identifier)
			problem.Internal(w, "failed to get profile")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}
