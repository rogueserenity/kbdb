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
	"github.com/rogueserenity/kbdb/internal/cascadedelete"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// parseListLimit reads the limit query param. The OpenAPI request validator
// (router.restOpenAPIValidator) enforces range (1-100) and injects the
// spec's default (api/openapi.yaml's Limit param) when absent, so it's
// always present and a valid integer here.
func parseListLimit(r *http.Request) int {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return limit
}

// ListSwitches reads the {userId} path value and lists that owner's
// switches. Anonymous callers are allowed; visibility is scoped to what the
// caller (if any) may read, per [authz.ReadableVisibilities].
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

		out, err := repoapi.SwitchToAPI(*sw)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to get switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
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
func validateSwitchLookups(ctx context.Context, w http.ResponseWriter, sw repository.Switch) (ok bool) {
	fieldErrs := lookup.ValidateSwitch(ctx, sw)
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
func CreateSwitch(switchRepo repository.SwitchRepository) http.HandlerFunc {
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

		if !validateSwitchLookups(r.Context(), w, sw) {
			return
		}

		sw.ID = uuid.NewString()

		created, err := switchRepo.Create(r.Context(), sw)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless.
			problem.Conflict(w, "switch already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating switch", log.Error, err, log.SwitchID, sw.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		out, err := repoapi.SwitchToAPI(*created)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, created.ID)
			problem.Internal(w, "failed to create switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's switch, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateSwitch(switchRepo repository.SwitchRepository) http.HandlerFunc {
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

		if !validateSwitchLookups(r.Context(), w, sw) {
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

		out, err := repoapi.SwitchToAPI(*updated)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping switch to API", log.Error, err, log.SwitchID, updated.ID)
			problem.Internal(w, "failed to update switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteSwitch reads the {userId} and {switchId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's switch returns 404, not 403, to avoid revealing it exists.
// The on_delete query param (default "block") controls what happens if a
// build still references this switch: see [cascadedelete.DeleteSwitch].
func DeleteSwitch(
	switchRepo repository.SwitchRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("switchId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be one of: block, cascade, detach")
			return
		}

		result, err := cascadedelete.DeleteSwitch(r.Context(), switchRepo, buildRepo, images, ownerID, id, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			problem.StillReferenced(w, "switch is still referenced by one or more builds", blocked.BuildIDs)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("deleting switch", log.Error, err, log.SwitchID, id)
			problem.Internal(w, "failed to delete switch")
			return
		}

		if onDelete == cascadedelete.OnDeleteCascade {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.CascadeDeleteResult{DeletedBuildIds: result.DeletedBuildIDs})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
