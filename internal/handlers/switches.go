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

// ListSwitches returns a handler for GET /v1/users/{userId}/switches. userId
// need not be the caller's own subject: the caller sees userId's switches
// whose visibility is readable to them, per internal/authz.
// middleware.OptionalAuth must run first so an anonymous caller is still
// permitted (limited to public switches) rather than rejected.
func ListSwitches(repo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		switches, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing switches", "error", err)
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

// GetSwitch returns a handler for GET /v1/users/{userId}/switches/{id}. Per
// api/openapi.yaml, a switch that exists but isn't owned by or shared with
// the caller returns 404, not 403 - this avoids revealing that an unshared
// item exists. middleware.OptionalAuth must run first, same as
// ListSwitches.
func GetSwitch(repo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("id")

		sw, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting switch", "error", err)
			problem.Internal(w, "failed to get switch")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, sw.Visibility) {
			problem.NotFound(w, "resource not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(repoapi.SwitchToAPI(*sw))
	}
}

// decodeSwitchInput reads the request body into a repository.Switch.
// Shape/required-field validation already happened in the OpenAPI request
// validator (internal/router.restOpenAPIValidator) before this handler ran.
// Open-vocabulary fields aren't checked here either - see
// validateSwitchLookups, which needs a repository.LookupRepository this
// function doesn't have.
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
	var checks []repository.FieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, repository.FieldCheck{Field: field, Value: *value, Category: category})
	}

	add("type", &sw.Type, repository.CategorySwitchType)
	add("material.top_housing", sw.Material.TopHousing, repository.CategorySwitchMaterial)
	add("material.bottom_housing", sw.Material.BottomHousing, repository.CategorySwitchMaterial)
	add("material.stem", sw.Material.Stem, repository.CategorySwitchMaterial)
	add("spring.material", sw.Spring.Material, repository.CategorySwitchSpringMaterial)
	add("purchase.vendor", sw.Purchase.Vendor, repository.CategoryVendor)

	fieldErrs, err := repository.ValidateFields(ctx, lookupRepo, checks)
	if err != nil {
		log.FromContext(ctx).Error("validating switch lookup fields", "error", err)
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

// CreateSwitch returns a handler for POST /v1/users/{userId}/switches. Per
// api/openapi.yaml, userId must be the caller's own subject - creating in
// another user's collection returns 404 (not 403, matching the read
// routes' anti-enumeration behavior), enforced via authz.IsOwner.
// middleware.Auth (not OptionalAuth) must run first: writes always require
// an authenticated caller.
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
			log.FromContext(r.Context()).Error("creating switch", "error", err)
			problem.Internal(w, "failed to create switch")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(repoapi.SwitchToAPI(*created))
	}
}

// DeleteSwitch returns a handler for DELETE /v1/users/{userId}/switches/{id}.
// Per api/openapi.yaml, userId must be the caller's own subject - deleting
// another user's switch returns 404 (anti-enumeration, via authz.IsOwner).
// Deleting the caller's own switch is idempotent - a nonexistent id is not
// an error, matching lookups' delete (see repository.SwitchRepository.
// Delete). middleware.Auth (not OptionalAuth) must run first: writes always
// require an authenticated caller.
func DeleteSwitch(switchRepo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("id")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		if err := switchRepo.Delete(r.Context(), id); err != nil {
			log.FromContext(r.Context()).Error("deleting switch", "error", err)
			problem.Internal(w, "failed to delete switch")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
