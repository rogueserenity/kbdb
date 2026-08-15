package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/cascadedelete"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ListKeyboards reads the {userId} path value and lists that owner's
// keyboards. Anonymous callers are allowed; visibility is scoped to what
// the caller (if any) may read, per [authz.ReadableVisibilities].
func ListKeyboards(repo repository.KeyboardRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		keyboards, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing keyboards", log.Error, err)
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

// GetKeyboard reads the {userId} and {keyboardId} path values. Anonymous callers
// are allowed; a keyboard that exists but isn't readable by the caller
// returns 404, not 403, to avoid revealing it exists.
func GetKeyboard(repo repository.KeyboardRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		kb, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting keyboard", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, kb.Visibility) {
			log.DeniedRead(r.Context(), "keyboard", ownerID, string(kb.Visibility), log.KeyboardID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		out, err := repoapi.KeyboardToAPI(*kb)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to get keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

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
func validateKeyboardLookups(ctx context.Context, w http.ResponseWriter, kb repository.Keyboard) (ok bool) {
	fieldErrs := lookup.ValidateKeyboard(ctx, kb)
	if len(fieldErrs) > 0 {
		invalidParams := make([]problem.InvalidParam, len(fieldErrs))
		for i, fe := range fieldErrs {
			invalidParams[i] = keyboardFieldErrorToInvalidParam(fe, kb.Size)
		}
		problem.ValidationFailed(w, "one or more fields are not approved lookup values", invalidParams)
		return false
	}

	return true
}

// keyboardFieldErrorToInvalidParam special-cases the layout/size cross-check
// (lookup.ValidateKeyboard reports it as a FieldError with Field "layout"
// and Category CategoryKeyboardSize) to keep its more specific message.
func keyboardFieldErrorToInvalidParam(fe lookup.FieldError, size *string) problem.InvalidParam {
	if fe.Field == "layout" && fe.Category == lookup.CategoryKeyboardSize {
		return problem.InvalidParam{
			Name:   fe.Field,
			Reason: fmt.Sprintf("%q is not a valid layout for size %q", fe.Value, *size),
		}
	}
	return problem.InvalidParam{
		Name:   fe.Field,
		Reason: fmt.Sprintf("%q is not an approved %s value", fe.Value, fe.Category),
	}
}

// CreateKeyboard reads the {userId} path value and requires an
// authenticated caller. userId must be the caller's own subject; creating
// in another user's collection returns 404, not 403, to avoid revealing it
// exists.
func CreateKeyboard(keyboardRepo repository.KeyboardRepository) http.HandlerFunc {
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

		if !validateKeyboardLookups(r.Context(), w, kb) {
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
			log.FromContext(r.Context()).Error("creating keyboard", log.Error, err, log.KeyboardID, kb.ID)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		out, err := repoapi.KeyboardToAPI(*created)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, created.ID)
			problem.Internal(w, "failed to create keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateKeyboard reads the {userId} and {keyboardId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's keyboard, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateKeyboard(keyboardRepo repository.KeyboardRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		kb, ok := decodeKeyboardInput(w, r)
		if !ok {
			return
		}

		if !validateKeyboardLookups(r.Context(), w, kb) {
			return
		}

		kb.ID = id

		updated, err := keyboardRepo.Update(r.Context(), kb)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("updating keyboard", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to update keyboard")
			return
		}

		out, err := repoapi.KeyboardToAPI(*updated)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keyboard to API", log.Error, err, log.KeyboardID, updated.ID)
			problem.Internal(w, "failed to update keyboard")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteKeyboard reads the {userId} and {keyboardId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's keyboard returns 404, not 403, to avoid revealing it
// exists. The on_delete query param (default "block") controls what
// happens if a build still references this keyboard: see
// [cascadedelete.DeleteKeyboard].
func DeleteKeyboard(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keyboardId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		onDelete, ok := cascadedelete.ParseOnDelete(r.URL.Query().Get("on_delete"))
		if !ok {
			problem.BadRequest(w, "on_delete must be one of: block, cascade, detach")
			return
		}

		result, err := cascadedelete.DeleteKeyboard(r.Context(), keyboardRepo, buildRepo, images, ownerID, id, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			problem.StillReferenced(w, "keyboard is still referenced by one or more builds", blocked.BuildIDs)
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("deleting keyboard", log.Error, err, log.KeyboardID, id)
			problem.Internal(w, "failed to delete keyboard")
			return
		}

		if onDelete == cascadedelete.OnDeleteCascade {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(api.CascadeDeleteResult{DeletedBuildIds: result.DeletedBuildIDs})
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
