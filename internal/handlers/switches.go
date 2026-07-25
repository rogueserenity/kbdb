package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

const (
	defaultSwitchListLimit = 20
	maxSwitchListLimit     = 100
)

// Lookup categories SwitchInput's open-vocabulary fields validate against,
// per their api/openapi.yaml descriptions ("validated against the ...
// lookup at request time"). Names match model/lookup_seed.json. type is not
// included: it's a closed enum in the openapi schema (Linear/Tactile/
// Clicky), checked directly rather than via a lookup - there's also a
// switch_type lookup category with the same three values, but the schema
// enum is what api/openapi.yaml actually declares as the contract.
const (
	switchMaterialCategory       = "switch_material"
	switchSpringMaterialCategory = "switch_spring_material"
	vendorCategory               = "vendor"
)

// switchSummary is the SwitchSummary schema in api/openapi.yaml: a subset
// of Switch's fields, returned by the list endpoint.
type switchSummary struct {
	ID    string `json:"id"`
	Brand string `json:"brand"`
	Name  string `json:"name"`
	Type  string `json:"type"`
}

type switchListPage struct {
	Items      []switchSummary `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

// parseListLimit reads the limit query param per api/openapi.yaml's Limit
// parameter (1-100, default 20), writing a 400 and returning ok=false if
// it's present but invalid.
func parseListLimit(w http.ResponseWriter, r *http.Request) (limit int, ok bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return defaultSwitchListLimit, true
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > maxSwitchListLimit {
		problem.BadRequest(w, "limit must be an integer between 1 and 100")
		return 0, false
	}

	return limit, true
}

// ListSwitches returns a handler for GET /users/{userId}/switches. userId
// need not be the caller's own subject: the caller sees userId's switches
// whose visibility is readable to them, per internal/authz.
// middleware.OptionalAuth must run first so an anonymous caller is still
// permitted (limited to public switches) rather than rejected.
func ListSwitches(repo repository.SwitchRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit, ok := parseListLimit(w, r)
		if !ok {
			return
		}
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		switches, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing switches", "error", err)
			problem.Internal(w, "failed to list switches")
			return
		}

		items := make([]switchSummary, len(switches))
		for i, sw := range switches {
			items[i] = switchSummary{ID: sw.ID, Brand: sw.Brand, Name: sw.Name, Type: sw.Type}
		}

		page := switchListPage{Items: items}
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
		_ = json.NewEncoder(w).Encode(sw)
	}
}

// decodeSwitchInput reads and shape-validates a request body against
// api/openapi.yaml's SwitchInput schema (required fields, type enum),
// writing a 400 and returning ok=false if the body is malformed or fails
// shape validation. Open-vocabulary fields (material.*, spring.material,
// purchase.vendor) aren't checked here - see validateSwitchLookups, which
// needs a repository.LookupRepository this function doesn't have.
func decodeSwitchInput(w http.ResponseWriter, r *http.Request) (sw repository.Switch, ok bool) {
	if err := json.NewDecoder(r.Body).Decode(&sw); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.Switch{}, false
	}

	if sw.Brand == "" || sw.Name == "" || sw.Type == "" {
		problem.BadRequest(w, "brand, name, and type are required")
		return repository.Switch{}, false
	}

	if !slices.Contains([]string{"Linear", "Tactile", "Clicky"}, sw.Type) {
		problem.BadRequest(w, "type must be one of Linear, Tactile, Clicky")
		return repository.Switch{}, false
	}

	if sw.Visibility == "" {
		sw.Visibility = repository.VisibilityPrivate
	} else if !sw.Visibility.Valid() {
		problem.BadRequest(w, "visibility must be one of public, authenticated, private")
		return repository.Switch{}, false
	}

	return sw, true
}

// validateSwitchLookups checks sw's open-vocabulary fields against their
// lookup categories (see the switch*Category/vendorCategory consts),
// writing a 400 naming the first invalid field and returning ok=false if
// any fails. A blank field is skipped (SwitchInput doesn't require these),
// not treated as invalid.
func validateSwitchLookups(ctx context.Context, w http.ResponseWriter, lookupRepo repository.LookupRepository, sw repository.Switch) (ok bool) {
	checks := []struct {
		field    string
		value    string
		category string
	}{
		{"material.top_housing", sw.Material.TopHousing, switchMaterialCategory},
		{"material.bottom_housing", sw.Material.BottomHousing, switchMaterialCategory},
		{"material.stem", sw.Material.Stem, switchMaterialCategory},
		{"spring.material", sw.Spring.Material, switchSpringMaterialCategory},
		{"purchase.vendor", sw.Purchase.Vendor, vendorCategory},
	}

	for _, c := range checks {
		if c.value == "" {
			continue
		}

		valid, err := lookupContains(ctx, lookupRepo, c.category, c.value)
		if err != nil {
			log.FromContext(ctx).Error("validating switch lookup field", "field", c.field, "error", err)
			problem.Internal(w, "failed to validate "+c.field)
			return false
		}
		if !valid {
			problem.BadRequest(w, fmt.Sprintf("%s: %q is not an approved %s value", c.field, c.value, c.category))
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
		if s, ok := v.(string); ok && s == value {
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
		_ = json.NewEncoder(w).Encode(created)
	}
}
