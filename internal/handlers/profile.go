package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/profileread"
	"github.com/rogueserenity/kbdb/internal/profilevalidate"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// validateProfileInput writes a 400 listing every field-level violation in
// p and returns false if any check fails, matching what the OpenAPI
// request validator enforces for the REST body. Shared with the MCP tool
// via internal/profilevalidate.
func validateProfileInput(w http.ResponseWriter, p repository.Profile) (ok bool) {
	fieldErrs := profilevalidate.Validate(p)
	if len(fieldErrs) == 0 {
		return true
	}

	invalidParams := make([]problem.InvalidParam, len(fieldErrs))
	for i, fe := range fieldErrs {
		invalidParams[i] = problem.InvalidParam{Name: fe.Name, Reason: fe.Reason}
	}
	problem.ValidationFailed(w, "one or more fields are invalid", invalidParams)

	return false
}

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

// CreateProfile reads the {identifier} path value (which for POST must be
// the caller's own IdP subject) and requires an authenticated caller.
// Anything that isn't the caller's own subject - a username, or another
// user's subject - returns 404, not 403, matching the don't-leak posture
// of the read route. A user has exactly one profile: a second create is 409
// (.../errors/conflict); a username already claimed by a different user is
// 409 with the distinct .../errors/username-unavailable type.
func CreateProfile(repo repository.ProfileRepository, images repository.ProfileImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("identifier")

		if !authz.IsOwner(r.Context(), userID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.ProfileInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		p := repoapi.ProfileToRepo(in)

		if !validateProfileInput(w, p) {
			return
		}

		created, err := repo.Create(r.Context(), p)
		switch {
		case errors.Is(err, repository.ErrAlreadyExists):
			problem.Conflict(w, "you already have a profile")
			return
		case errors.Is(err, repository.ErrUsernameTaken):
			problem.UsernameUnavailable(w, fmt.Sprintf("the username %q is already taken", p.Username))
			return
		case err != nil:
			log.FromContext(r.Context()).Error("creating profile", log.Error, err)
			problem.Internal(w, "failed to create profile")
			return
		}

		out, err := repoapi.ProfileToAPI(r.Context(), *created, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping profile to API", log.Error, err)
			problem.Internal(w, "failed to create profile")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}
