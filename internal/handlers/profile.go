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

// validateProfileInput writes a 400 listing every field-level violation and
// returns false if any check fails.
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

// GetProfile returns the profile for {identifier} (an IdP subject or a
// username). Anonymous callers are allowed. A non-discoverable profile, or
// an identifier matching nothing, is 404 (not 403) for anyone but the owner.
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

// CreateProfile creates the caller's profile. {identifier} must be the
// caller's own subject (anything else is 404, not 403). A second create is
// 409 .../errors/conflict; a username taken by another user is 409
// .../errors/username-unavailable.
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

// UpdateProfile replaces the caller's profile. {identifier} must be the
// caller's own subject, and a caller with no profile yet is 404 (both not
// 403). Full replace: an omitted body field is cleared, the avatar
// untouched. A username taken by another user is 409
// .../errors/username-unavailable.
func UpdateProfile(repo repository.ProfileRepository, images repository.ProfileImageStore) http.HandlerFunc {
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

		updated, err := repo.Update(r.Context(), p)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			problem.NotFound(w, "resource not found")
			return
		case errors.Is(err, repository.ErrUsernameTaken):
			problem.UsernameUnavailable(w, fmt.Sprintf("the username %q is already taken", p.Username))
			return
		case err != nil:
			log.FromContext(r.Context()).Error("updating profile", log.Error, err)
			problem.Internal(w, "failed to update profile")
			return
		}

		out, err := repoapi.ProfileToAPI(r.Context(), *updated, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping profile to API", log.Error, err)
			problem.Internal(w, "failed to update profile")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}
