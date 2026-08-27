package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

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

// ListProfiles returns a page of discoverable profiles. username /
// discord_username are mutually-exclusive begins-with filters (both is a
// 400); a next_cursor can't be reused across filters (also a 400).
// Anonymous callers are allowed.
func ListProfiles(repo repository.ProfileRepository, images repository.ProfileImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")
		usernamePrefix := r.URL.Query().Get("username")
		discordPrefix := r.URL.Query().Get("discord_username")

		if usernamePrefix != "" && discordPrefix != "" {
			problem.BadRequest(w, "username and discord_username are mutually exclusive")
			return
		}

		profiles, nextCursor, err := repo.ListPublic(r.Context(), usernamePrefix, discordPrefix, limit, cursor)
		if errors.Is(err, repository.ErrInvalidCursor) {
			problem.BadRequest(w, "invalid pagination cursor; restart from the first page")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing profiles", log.Error, err)
			problem.Internal(w, "failed to list profiles")
			return
		}

		items := make([]api.ProfileSummary, len(profiles))
		errs := make([]error, len(profiles))

		ctx := r.Context()
		var wg sync.WaitGroup
		for i, p := range profiles {
			wg.Add(1)
			go func(i int, p repository.Profile) {
				defer wg.Done()

				summary, err := repoapi.ProfileToAPISummary(ctx, p, images)
				if err != nil {
					errs[i] = fmt.Errorf("mapping profile %q to API summary: %w", p.Username, err)
					return
				}
				items[i] = summary
			}(i, p)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(r.Context()).Error("mapping profiles to API summaries", log.Error, err)
			problem.Internal(w, "failed to list profiles")
			return
		}

		page := api.ProfileListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
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
		case errors.Is(err, repository.ErrUsernameTaken):
			problem.UsernameUnavailable(w, fmt.Sprintf("the username %q is already taken", p.Username))
			return
		case handleMutationError(w, r, err, log.ProfileID, userID):
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

// DeleteProfile removes the caller's profile. {identifier} must be the
// caller's own subject (anything else is 404, not 403). A leaf delete -
// nothing references a Profile - so no cascade; it only makes the user
// undiscoverable. If an avatar is on file, its object is removed first
// (DB-then-S3 ordering, matching the single-image delete policy in
// internal/repository); a failed object delete aborts before the DB
// delete. Idempotent: no profile is still 204.
func DeleteProfile(repo repository.ProfileRepository, images repository.ProfileImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.PathValue("identifier")

		if !authz.IsOwner(r.Context(), userID) {
			problem.NotFound(w, "resource not found")
			return
		}

		p, err := repo.Get(r.Context(), userID)
		if errors.Is(err, repository.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting profile", log.Error, err, log.ProfileID, userID)
			problem.Internal(w, "failed to delete profile")
			return
		}

		if p.AvatarPath != nil {
			if err := images.Delete(r.Context(), *p.AvatarPath); err != nil {
				log.FromContext(r.Context()).Error("deleting profile avatar object", log.Error, err, log.ProfileID, userID)
				problem.Internal(w, "failed to delete profile")
				return
			}
		}

		if handleMutationError(w, r, repo.Delete(r.Context()), log.ProfileID, userID) {
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
