package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// requireNonBlankCategory writes a 400 and returns false if category is
// empty or whitespace-only.
func requireNonBlankCategory(w http.ResponseWriter, category string) bool {
	if strings.TrimSpace(category) == "" {
		problem.BadRequest(w, "category must not be blank")
		return false
	}
	return true
}

// decodeValues reads and validates a request body against
// api/openapi.yaml's LookupInput schema, writing a 400 and returning
// ok=false if the body is malformed or values is empty.
func decodeValues(w http.ResponseWriter, r *http.Request) (values []any, ok bool) {
	var input struct {
		Values []any `json:"values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		problem.BadRequest(w, "invalid request body")
		return nil, false
	}
	if len(input.Values) == 0 {
		problem.BadRequest(w, "values must not be empty")
		return nil, false
	}
	return input.Values, true
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(lookup)
	}
}

// putLookup backs CreateLookup and ReplaceLookup, which differ only in
// which repo method to call, which sentinel error maps to an expected
// (non-500) problem response, and the success status code. verb
// ("creating"/"replacing") and infinitive ("create"/"replace") keep the
// two callers' log and error messages grammatically correct.
func putLookup(
	verb, infinitive string,
	call func(ctx context.Context, category string, values []any) (*repository.Lookup, error),
	expectedErr error,
	writeExpectedErr func(w http.ResponseWriter),
	successStatus int,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")
		if !requireNonBlankCategory(w, category) {
			return
		}

		values, ok := decodeValues(w, r)
		if !ok {
			return
		}

		lookup, err := call(r.Context(), category, values)
		if errors.Is(err, expectedErr) {
			writeExpectedErr(w)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error(verb+" lookup category", "error", err)
			problem.Internal(w, "failed to "+infinitive+" lookup category")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(successStatus)
		_ = json.NewEncoder(w).Encode(lookup)
	}
}

// CreateLookup returns a handler for POST /v1/lookups/{category}: create a
// new lookup category (admin only - see middleware.RequireAdmin).
func CreateLookup(repo repository.LookupRepository) http.HandlerFunc {
	return putLookup("creating", "create", repo.CreateCategory, repository.ErrAlreadyExists,
		func(w http.ResponseWriter) { problem.Conflict(w, "category already exists") },
		http.StatusCreated)
}

// ReplaceLookup returns a handler for PUT /v1/lookups/{category}: replace
// an existing lookup category's approved values (admin only - see
// middleware.RequireAdmin).
func ReplaceLookup(repo repository.LookupRepository) http.HandlerFunc {
	return putLookup("replacing", "replace", repo.ReplaceCategory, repository.ErrNotFound,
		func(w http.ResponseWriter) { problem.NotFound(w, "resource not found") },
		http.StatusOK)
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
