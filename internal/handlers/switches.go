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

const defaultSwitchListLimit = 20

// Lookup categories SwitchInput's open-vocabulary fields validate against,
// per their api/openapi.yaml descriptions ("validated against the ...
// lookup at request time"). Names match model/lookup_seed.json.
const (
	switchTypeCategory           = "switch_type"
	switchMaterialCategory       = "switch_material"
	switchSpringMaterialCategory = "switch_spring_material"
	vendorCategory               = "vendor"
)

// parseListLimit reads the limit query param, defaulting when absent. Range
// (1-100) and type are enforced by the OpenAPI request validator
// (internal/router.restOpenAPIValidator) before this handler runs, so a
// present value is always a valid integer here.
func parseListLimit(r *http.Request) int {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultSwitchListLimit
	}

	limit, _ := strconv.Atoi(raw)

	return limit
}

// ListSwitches returns a handler for GET /users/{userId}/switches. userId
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

// GetSwitch returns a handler for GET /users/{userId}/switches/{id}. Per
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

// validateSwitchLookups writes a 400 naming the first invalid field and
// returns ok=false if any check fails. An unset (nil) field is skipped, not
// treated as invalid - SwitchInput doesn't require these.
func validateSwitchLookups(ctx context.Context, w http.ResponseWriter, lookupRepo repository.LookupRepository, sw repository.Switch) (ok bool) {
	checks := []struct {
		field    string
		value    *string
		category string
	}{
		{"type", &sw.Type, switchTypeCategory},
		{"material.top_housing", sw.Material.TopHousing, switchMaterialCategory},
		{"material.bottom_housing", sw.Material.BottomHousing, switchMaterialCategory},
		{"material.stem", sw.Material.Stem, switchMaterialCategory},
		{"spring.material", sw.Spring.Material, switchSpringMaterialCategory},
		{"purchase.vendor", sw.Purchase.Vendor, vendorCategory},
	}

	for _, c := range checks {
		if c.value == nil {
			continue
		}

		valid, err := lookupContains(ctx, lookupRepo, c.category, *c.value)
		if err != nil {
			log.FromContext(ctx).Error("validating switch lookup field", "field", c.field, "error", err)
			problem.Internal(w, "failed to validate "+c.field)
			return false
		}
		if !valid {
			problem.BadRequest(w, fmt.Sprintf("%s: %q is not an approved %s value", c.field, *c.value, c.category))
			return false
		}
	}

	return true
}

// lookupContains reports whether value is one of category's approved
// values. category not existing is not itself an error here (it just means
// nothing validates) - CreateSwitch treats that the same as "not found" via
// the false return, since either way value can't be approved.
func lookupContains(ctx context.Context, lookupRepo repository.LookupRepository, category, value string) (bool, error) {
	lookup, err := lookupRepo.GetCategory(ctx, category)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	for _, v := range lookup.Values {
		s, ok := v.(string)
		if !ok {
			// repository.Lookup.Values is []any because some categories
			// (e.g. build_case_mount_type) store objects, not strings - a
			// non-string entry here means this category's data isn't
			// shaped the way switches validation expects, not that value
			// is unapproved. Log it so that's discoverable rather than
			// looking like an ordinary rejection.
			log.FromContext(ctx).Warn("lookup category has non-string value", "category", category)
			continue
		}
		if s == value {
			return true, nil
		}
	}

	return false, nil
}

// CreateSwitch returns a handler for POST /users/{userId}/switches. Per
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

		sw.UserID = ownerID
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
