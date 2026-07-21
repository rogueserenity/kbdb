package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

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
