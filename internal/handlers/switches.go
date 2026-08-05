package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// parseListLimit reads the limit query param. The OpenAPI request validator
// (internal/router.restOpenAPIValidator) enforces range (1-100) and injects
// the spec's default (api/openapi.yaml's Limit param) when absent, so it's
// always present and a valid integer here.
func parseListLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return limit
}

// ListSwitches reads the {userId} path value and lists that owner's
// switches. Anonymous callers are allowed; visibility is scoped to what the
// caller (if any) may read, per internal/authz.
func ListSwitches(repo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		switches, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing switches", log.Error, err)
			problem.Internal(w, "failed to list switches")
			return
		}

		items := make([]api.SwitchSummary, len(switches))
		for i, sw := range switches {
			items[i] = repoapi.SwitchToAPISummary(sw)
		}

		page := api.SwitchListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetSwitch reads the {userId} and {switchId} path values. Anonymous callers are
// allowed; a switch that exists but isn't readable by the caller returns
// 404, not 403, to avoid revealing it exists.
func GetSwitch(repo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		sw, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to get switch")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, sw.Visibility) {
			log.DeniedRead(r.Context(), "switch", ownerID, string(sw.Visibility), log.SwitchID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(repoapi.SwitchToAPI(*sw))
	}
}

func decodeSwitchInput(w http.ResponseWriter, r *http.Request) (sw repository.Switch, ok bool) {
	var in api.SwitchInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.Switch{}, false
	}

	sw = repoapi.SwitchToRepo(in)

	return sw, true
}

// validateSwitchLookups writes a 400 listing every invalid field if any
// check fails. An unset (nil) field is skipped, not treated as invalid.
func validateSwitchLookups(ctx context.Context, w http.ResponseWriter, lookupRepo repository.LookupRepository, sw repository.Switch) (ok bool) {
	fieldErrs, err := lookup.ValidateSwitch(ctx, lookupRepo, sw)
	if err != nil {
		log.FromContext(ctx).Error("validating switch lookup fields", log.Error, err)
		problem.Internal(w, "failed to validate lookup fields")
		return false
	}
	if len(fieldErrs) > 0 {
		invalidParams := make([]problem.InvalidParam, len(fieldErrs))
		for i, fe := range fieldErrs {
			invalidParams[i] = problem.InvalidParam{
				Name:   fe.Field,
				Reason: fmt.Sprintf("%q is not an approved %s value", fe.Value, fe.Category),
			}
		}
		problem.ValidationFailed(w, "one or more fields are not approved lookup values", invalidParams)
		return false
	}

	return true
}

// CreateSwitch reads the {userId} path value and requires an authenticated
// caller. userId must be the caller's own subject; creating in another
// user's collection returns 404, not 403, to avoid revealing it exists.
func CreateSwitch(switchRepo repository.SwitchRepository, lookupRepo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		sw, ok := decodeSwitchInput(w, r)
		if !ok {
			return
		}

		if !validateSwitchLookups(r.Context(), w, lookupRepo, sw) {
			return
		}

		sw.ID = uuid.NewString()

		created, err := switchRepo.Create(r.Context(), sw)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateLookup does.
			problem.Conflict(w, "switch already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating switch", log.Error, err, log.SwitchID, sw.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(repoapi.SwitchToAPI(*created))
	}
}

// UpdateSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's switch, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateSwitch(switchRepo repository.SwitchRepository, lookupRepo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		sw, ok := decodeSwitchInput(w, r)
		if !ok {
			return
		}

		if !validateSwitchLookups(r.Context(), w, lookupRepo, sw) {
			return
		}

		sw.ID = id

		updated, err := switchRepo.Update(r.Context(), sw)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("updating switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to update switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(repoapi.SwitchToAPI(*updated))
	}
}

// DeleteSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's switch returns 404, not 403, to avoid revealing it exists.
func DeleteSwitch(switchRepo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		if err := switchRepo.Delete(r.Context(), id); err != nil {
			log.FromContext(r.Context()).Error("deleting switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to delete switch")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
