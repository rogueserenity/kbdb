package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

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

// CreateLookup returns a handler for POST /v1/lookups/{category}: create a
// new lookup category (admin only - see middleware.RequireAdmin).
func CreateLookup(repo repository.LookupRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		category := r.PathValue("category")
		if strings.TrimSpace(category) == "" {
			problem.BadRequest(w, "category must not be blank")
			return
		}

		var input struct {
			Values []any `json:"values"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}
		if len(input.Values) == 0 {
			problem.BadRequest(w, "values must not be empty")
			return
		}

		lookup, err := repo.CreateCategory(r.Context(), category, input.Values)
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
		_ = json.NewEncoder(w).Encode(lookup)
	}
}
