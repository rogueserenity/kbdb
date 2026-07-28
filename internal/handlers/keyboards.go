package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ListKeyboards reads the {userId} path value and lists that owner's
// keyboards. Anonymous callers are allowed; visibility is scoped to what
// the caller (if any) may read, per internal/authz.
func ListKeyboards(repo repository.KeyboardRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		keyboards, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing keyboards", "error", err)
			problem.Internal(w, "failed to list keyboards")
			return
		}

		items := make([]api.KeyboardSummary, len(keyboards))
		for i, kb := range keyboards {
			items[i] = repoapi.KeyboardToAPISummary(kb)
		}

		page := api.KeyboardListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetKeyboard reads the {userId} and {id} path values. Anonymous callers
// are allowed; a keyboard that exists but isn't readable by the caller
// returns 404, not 403, to avoid revealing it exists.
func GetKeyboard(repo repository.KeyboardRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("id")

		kb, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting keyboard", "error", err)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, kb.Visibility) {
			problem.NotFound(w, "resource not found")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(repoapi.KeyboardToAPI(*kb))
	}
}

// decodeKeyboardInput reads the request body into a repository.Keyboard.
// Shape/required-field validation already happened in the OpenAPI request
// validator (internal/router.restOpenAPIValidator) before this handler ran.
// Open-vocabulary fields aren't checked here either - see
// validateKeyboardLookups, which needs a repository.LookupRepository this
// function doesn't have.
func decodeKeyboardInput(w http.ResponseWriter, r *http.Request) (kb repository.Keyboard, ok bool) {
	var in api.KeyboardInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.Keyboard{}, false
	}

	kb = repoapi.KeyboardToRepo(in)

	return kb, true
}

// validateKeyboardLookups writes a 400 listing every invalid field if any
// check fails. An unset (nil) field is skipped, not treated as invalid.
func validateKeyboardLookups(ctx context.Context, w http.ResponseWriter, lookupRepo repository.LookupRepository, kb repository.Keyboard) (ok bool) {
	var checks []repository.FieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, repository.FieldCheck{Field: field, Value: *value, Category: category})
	}

	add("size", kb.Size, repository.CategoryKeyboardSize)
	add("design.top_case.material", kb.Design.TopCase.Material, repository.CategoryKeyboardCaseMaterial)
	add("design.bottom_case.material", kb.Design.BottomCase.Material, repository.CategoryKeyboardCaseMaterial)
	add("design.weight.material", kb.Design.Weight.Material, repository.CategoryKeyboardWeightMaterial)
	add("pcb.firmware", kb.PCB.Firmware, repository.CategoryKeyboardPCBFirmware)
	add("pcb.assembly", kb.PCB.Assembly, repository.CategoryKeyboardPCBAssemblyType)
	add("pcb.connectivity", kb.PCB.Connectivity, repository.CategoryKeyboardPCBConnectivityType)
	add("purchase.vendor", kb.Purchase.Vendor, repository.CategoryVendor)
	add("purchase.order_status", kb.Purchase.OrderStatus, repository.CategoryOrderStatus)

	for i, material := range kb.Design.Plates {
		checks = append(checks, repository.FieldCheck{
			Field:    fmt.Sprintf("design.plates[%d]", i),
			Value:    material,
			Category: repository.CategoryKeyboardPlateMaterial,
		})
	}

	fieldErrs, err := repository.ValidateFields(ctx, lookupRepo, checks)
	if err != nil {
		log.FromContext(ctx).Error("validating keyboard lookup fields", "error", err)
		problem.Internal(w, "failed to validate lookup fields")
		return false
	}

	var invalidParams []problem.InvalidParam
	for _, fe := range fieldErrs {
		invalidParams = append(invalidParams, problem.InvalidParam{
			Name:   fe.Field,
			Reason: fmt.Sprintf("%q is not an approved %s value", fe.Value, fe.Category),
		})
	}

	if kb.Layout != nil {
		layoutErr, err := validateKeyboardLayout(ctx, lookupRepo, kb.Size, *kb.Layout)
		if err != nil {
			log.FromContext(ctx).Error("validating keyboard layout", "error", err)
			problem.Internal(w, "failed to validate lookup fields")
			return false
		}
		if layoutErr != nil {
			invalidParams = append(invalidParams, *layoutErr)
		}
	}

	if len(invalidParams) > 0 {
		problem.ValidationFailed(w, "one or more fields are not approved lookup values", invalidParams)
		return false
	}

	return true
}

// validateKeyboardLayout skips the size-membership check when size is nil,
// since size is independently optional.
func validateKeyboardLayout(
	ctx context.Context,
	lookupRepo repository.LookupRepository,
	size *string,
	layout string,
) (*problem.InvalidParam, error) {
	category := repository.CategoryKeyboardLayout

	lookup, err := lookupRepo.GetCategory(ctx, category)
	if errors.Is(err, repository.ErrNotFound) {
		return &problem.InvalidParam{
			Name:   "layout",
			Reason: fmt.Sprintf("%q is not an approved %s value", layout, category),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	values, err := repository.ParseLayoutValues(lookup.Values)
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(values, func(v repository.LayoutValue) bool { return v.Name == layout })
	if idx == -1 {
		return &problem.InvalidParam{
			Name:   "layout",
			Reason: fmt.Sprintf("%q is not an approved %s value", layout, category),
		}, nil
	}

	if size != nil && !slices.Contains(values[idx].Sizes, *size) {
		return &problem.InvalidParam{
			Name:   "layout",
			Reason: fmt.Sprintf("%q is not a valid layout for size %q", layout, *size),
		}, nil
	}

	return nil, nil //nolint:nilnil // no problem found is a valid, expected result
}

// CreateKeyboard reads the {userId} path value and requires an
// authenticated caller. userId must be the caller's own subject; creating
// in another user's collection returns 404, not 403, to avoid revealing it
// exists.
func CreateKeyboard(keyboardRepo repository.KeyboardRepository, lookupRepo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		kb, ok := decodeKeyboardInput(w, r)
		if !ok {
			return
		}

		if !validateKeyboardLookups(r.Context(), w, lookupRepo, kb) {
			return
		}

		kb.ID = uuid.NewString()

		created, err := keyboardRepo.Create(r.Context(), kb)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateSwitch does.
			problem.Conflict(w, "keyboard already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating keyboard", "error", err)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(repoapi.KeyboardToAPI(*created))
	}
}
