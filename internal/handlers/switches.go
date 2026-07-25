package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repository"
)

const (
	defaultSwitchListLimit = 20
	maxSwitchListLimit     = 100
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
