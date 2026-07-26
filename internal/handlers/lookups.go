package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
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
		_ = json.NewEncoder(w).Encode(repoapi.LookupToAPI(*lookup))
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(repoapi.LookupToAPI(*lookup))
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(repoapi.LookupToAPI(*lookup))
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
