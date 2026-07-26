package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// requireNonBlankCategory writes a 400 and returns false if category is
// empty or whitespace-only. Not schema-enforceable: net/http's own
// {category} routing already collapses a whitespace-only path segment to
// an empty PathValue before the OpenAPI validator ever sees it.
func requireNonBlankCategory(w http.ResponseWriter, category string) bool {
	if strings.TrimSpace(category) == "" {
		problem.BadRequest(w, "category must not be blank")
		return false
	}
	return true
}

// decodeValues reads the request body into a []any. Shape/required-field
// validation (values present and non-empty) already happened in the
// OpenAPI request validator (internal/router.restOpenAPIValidator) before
// this handler ran.
func decodeValues(w http.ResponseWriter, r *http.Request) (values []any, ok bool) {
	var in api.LookupInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return nil, false
	}

	return repoapi.LookupInputToRepo(in), true
}

// validateLookupValues checks that values is shaped the way category
// requires, writing a 400 and returning false if not.
func validateLookupValues(ctx context.Context, w http.ResponseWriter, repo repository.LookupRepository, category string, values []any) (ok bool) {
	switch category {
	case repository.CategoryKeyboardLayout:
		return validateKeyboardLayoutValues(ctx, w, repo, values)
	case repository.CategoryBuildCaseMountType:
		if _, err := repository.ParseCaseMountTypeValues(values); err != nil {
			problem.BadRequest(w, fmt.Sprintf("values: %s", err))
			return false
		}
	default:
		if _, err := repository.ParseStrings(values); err != nil {
			problem.BadRequest(w, fmt.Sprintf("values: %s", err))
			return false
		}
	}

	return true
}

// validateKeyboardLayoutValues also cross-checks each entry's sizes against
// CategoryKeyboardSize, so a keyboard can't later pass its layout-vs-size
// check against a size that was never itself approved.
func validateKeyboardLayoutValues(ctx context.Context, w http.ResponseWriter, repo repository.LookupRepository, values []any) (ok bool) {
	layouts, err := repository.ParseLayoutValues(values)
	if err != nil {
		problem.BadRequest(w, fmt.Sprintf("values: %s", err))
		return false
	}

	sizeLookup, err := repo.GetCategory(ctx, repository.CategoryKeyboardSize)
	var approvedSizes []string
	switch {
	case errors.Is(err, repository.ErrNotFound):
		// approvedSizes stays nil - every non-empty sizes list fails below.
	case err != nil:
		log.FromContext(ctx).Error("fetching keyboard_size for keyboard_layout validation", "error", err)
		problem.Internal(w, "failed to validate values")
		return false
	default:
		approvedSizes, err = repository.ParseStrings(sizeLookup.Values)
		if err != nil {
			log.FromContext(ctx).Error("parsing keyboard_size values", "error", err)
			problem.Internal(w, "failed to validate values")
			return false
		}
	}

	var invalidParams []problem.InvalidParam
	for _, l := range layouts {
		for _, size := range l.Sizes {
			if !slices.Contains(approvedSizes, size) {
				invalidParams = append(invalidParams, problem.InvalidParam{
					Name:   fmt.Sprintf("values[%s].sizes", l.Name),
					Reason: fmt.Sprintf("%q is not an approved keyboard_size value", size),
				})
			}
		}
	}
	if len(invalidParams) > 0 {
		problem.ValidationFailed(w, "one or more layout sizes are not approved keyboard_size values", invalidParams)
		return false
	}

	return true
}

// ListLookups returns a handler for GET /v1/lookups: all lookup category
// names, not their values (see GET /v1/lookups/{category} for that).
func ListLookups(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categories, err := repo.ListCategories(r.Context())
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("listing lookup categories", "error", err)
			problem.Internal(w, "failed to list lookup categories")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(categories)
	}
}

// writeLookup writes lookup as the response body, or a 500 if its stored
// values don't match the shape its category expects.
func writeLookup(ctx context.Context, w http.ResponseWriter, status int, lookup repository.Lookup) {
	out, err := repoapi.LookupToAPI(lookup)
	if err != nil {
		log.FromContext(ctx).Error("mapping lookup category to API shape", "category", lookup.Category, "error", err)
		problem.Internal(w, "failed to read lookup category")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(out)
}

// GetLookup returns a handler for GET /v1/lookups/{category}: one lookup
// category's approved values.
func GetLookup(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")

		lookup, err := repo.GetCategory(r.Context(), category)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting lookup category", "error", err)
			problem.Internal(w, "failed to get lookup category")
			return
		}

		writeLookup(r.Context(), w, http.StatusOK, *lookup)
	}
}

// CreateLookup returns a handler for POST /v1/lookups/{category}: create a
// new lookup category (admin only - see middleware.RequireAdmin).
func CreateLookup(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")
		if !requireNonBlankCategory(w, category) {
			return
		}

		values, ok := decodeValues(w, r)
		if !ok {
			return
		}

		if !validateLookupValues(r.Context(), w, repo, category, values) {
			return
		}

		lookup, err := repo.CreateCategory(r.Context(), category, values)
		if errors.Is(err, repository.ErrAlreadyExists) {
			problem.Conflict(w, "category already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating lookup category", "error", err)
			problem.Internal(w, "failed to create lookup category")
			return
		}

		writeLookup(r.Context(), w, http.StatusCreated, *lookup)
	}
}

// ReplaceLookup returns a handler for PUT /v1/lookups/{category}: replace
// an existing lookup category's approved values (admin only - see
// middleware.RequireAdmin).
func ReplaceLookup(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")
		if !requireNonBlankCategory(w, category) {
			return
		}

		values, ok := decodeValues(w, r)
		if !ok {
			return
		}

		if !validateLookupValues(r.Context(), w, repo, category, values) {
			return
		}

		lookup, err := repo.ReplaceCategory(r.Context(), category, values)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("replacing lookup category", "error", err)
			problem.Internal(w, "failed to replace lookup category")
			return
		}

		writeLookup(r.Context(), w, http.StatusOK, *lookup)
	}
}

// DeleteLookup returns a handler for DELETE /v1/lookups/{category}: delete
// a lookup category (admin only - see middleware.RequireAdmin). Idempotent:
// returns 204 whether or not the category existed.
func DeleteLookup(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")
		if !requireNonBlankCategory(w, category) {
			return
		}

		if err := repo.DeleteCategory(r.Context(), category); err != nil {
			log.FromContext(r.Context()).Error("deleting lookup category", "error", err)
			problem.Internal(w, "failed to delete lookup category")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
