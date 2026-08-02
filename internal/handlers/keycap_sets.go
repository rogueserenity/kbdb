package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ListKeycapSets reads the {userId} path value and lists that owner's
// keycap sets. Anonymous callers are allowed; visibility is scoped to what
// the caller (if any) may read, per internal/authz.
func ListKeycapSets(repo repository.KeycapSetRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		sets, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing keycap sets", log.Error, err)
			problem.Internal(w, "failed to list keycap sets")
			return
		}

		items := make([]api.KeycapSetSummary, len(sets))
		for i, ks := range sets {
			items[i] = repoapi.KeycapSetToAPISummary(ks)
		}

		page := api.KeycapSetListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetKeycapSet reads the {userId} and {keycapSetId} path values. Anonymous callers
// are allowed; a keycap set that exists but isn't readable by the caller
// returns 404, not 403, to avoid revealing it exists.
func GetKeycapSet(repo repository.KeycapSetRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keycapSetId")

		ks, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting keycap set", log.Error, err, log.KeycapSetID, id)
			problem.Internal(w, "failed to get keycap set")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, ks.Visibility) {
			problem.NotFound(w, "resource not found")
			return
		}

		out, err := repoapi.KeycapSetToAPI(r.Context(), *ks, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keycap set to API", log.Error, err, log.KeycapSetID, id)
			problem.Internal(w, "failed to get keycap set")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

func decodeKeycapSetInput(w http.ResponseWriter, r *http.Request) (ks repository.KeycapSet, ok bool) {
	var in api.KeycapSetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		problem.BadRequest(w, "invalid request body")
		return repository.KeycapSet{}, false
	}

	return repoapi.KeycapSetToRepo(in), true
}

// validateKeycapSetLookups writes a 400 listing every invalid field if any
// check fails. An unset (nil) field is skipped, not treated as invalid.
func validateKeycapSetLookups(ctx context.Context, w http.ResponseWriter, lookupRepo repository.LookupRepository, ks repository.KeycapSet) (ok bool) {
	var checks []repository.FieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, repository.FieldCheck{Field: field, Value: *value, Category: category})
	}

	add("profile", ks.Profile, repository.CategoryKeycapProfile)
	add("material", ks.Material, repository.CategoryKeycapMaterial)

	fieldErrs, err := repository.ValidateFields(ctx, lookupRepo, checks)
	if err != nil {
		log.FromContext(ctx).Error("validating keycap set lookup fields", log.Error, err)
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

	if len(invalidParams) > 0 {
		problem.ValidationFailed(w, "one or more fields are not approved lookup values", invalidParams)
		return false
	}

	return true
}

// CreateKeycapSet reads the {userId} path value and requires an
// authenticated caller. userId must be the caller's own subject; creating
// in another user's collection returns 404, not 403, to avoid revealing it
// exists.
func CreateKeycapSet(keycapSetRepo repository.KeycapSetRepository, lookupRepo repository.LookupRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		ks, ok := decodeKeycapSetInput(w, r)
		if !ok {
			return
		}

		if !validateKeycapSetLookups(r.Context(), w, lookupRepo, ks) {
			return
		}

		ks.ID = uuid.NewString()

		created, err := keycapSetRepo.Create(r.Context(), ks)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateKeyboard does.
			problem.Conflict(w, "keycap set already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating keycap set", log.Error, err, log.KeycapSetID, ks.ID)
			problem.Internal(w, "failed to create keycap set")
			return
		}

		out, err := repoapi.KeycapSetToAPI(r.Context(), *created, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keycap set to API", log.Error, err, log.KeycapSetID, created.ID)
			problem.Internal(w, "failed to create keycap set")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateKeycapSet reads the {userId} and {keycapSetId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's keycap set, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateKeycapSet(keycapSetRepo repository.KeycapSetRepository, lookupRepo repository.LookupRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keycapSetId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		ks, ok := decodeKeycapSetInput(w, r)
		if !ok {
			return
		}

		if !validateKeycapSetLookups(r.Context(), w, lookupRepo, ks) {
			return
		}

		ks.ID = id

		updated, err := keycapSetRepo.Update(r.Context(), ks)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			// Warn, not Error: expected contention under retry, not a bug -
			// still worth a trace if one set sees this repeatedly.
			log.FromContext(r.Context()).Warn("keycap set mutation conflict", log.KeycapSetID, id)
			problem.Conflict(w, "the keycap set is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("updating keycap set", log.Error, err, log.KeycapSetID, id)
			problem.Internal(w, "failed to update keycap set")
			return
		}

		out, err := repoapi.KeycapSetToAPI(r.Context(), *updated, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keycap set to API", log.Error, err, log.KeycapSetID, updated.ID)
			problem.Internal(w, "failed to update keycap set")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteKeycapSet reads the {userId} and {keycapSetId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's keycap set returns 404, not 403, to avoid revealing it
// exists.
func DeleteKeycapSet(keycapSetRepo repository.KeycapSetRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("keycapSetId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		if err := keycapSetRepo.Delete(r.Context(), id); err != nil {
			log.FromContext(r.Context()).Error("deleting keycap set", log.Error, err, log.KeycapSetID, id)
			problem.Internal(w, "failed to delete keycap set")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// CreateKeycapKit reads the {userId} and {keycapSetId} (parent set) path values and
// requires an authenticated caller. Kits have no independent visibility -
// authorization is entirely the parent set's ownership. userId must be the
// caller's own subject; adding a kit to another user's set, or to a set
// that doesn't exist, both return 404, to avoid revealing it exists.
func CreateKeycapKit(keycapSetRepo repository.KeycapSetRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		setID := r.PathValue("keycapSetId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeycapKitInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		kit := repoapi.KeycapKitToRepo(in)
		kit.KitID = uuid.NewString()

		created, err := keycapSetRepo.AddKit(r.Context(), setID, kit)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			// Warn, not Error: expected contention under retry, not a bug -
			// still worth a trace if one set sees this repeatedly.
			log.FromContext(r.Context()).Warn("keycap set mutation conflict", log.KeycapSetID, setID, log.KeycapKitID, kit.KitID)
			problem.Conflict(w, "the keycap set is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("adding keycap kit", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, kit.KitID)
			problem.Internal(w, "failed to add kit")
			return
		}

		out, err := repoapi.KeycapKitToAPI(r.Context(), *created, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keycap kit to API", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, created.KitID)
			problem.Internal(w, "failed to add kit")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateKeycapKit reads the {userId}, {keycapSetId} (parent set), and {kitId} path
// values and requires an authenticated caller. Kits have no independent
// visibility - authorization is entirely the parent set's ownership.
// userId must be the caller's own subject; updating a kit on another
// user's set, a set that doesn't exist, or a kitId that doesn't exist
// within it, all return 404, to avoid revealing it exists.
func UpdateKeycapKit(keycapSetRepo repository.KeycapSetRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		setID := r.PathValue("keycapSetId")
		kitID := r.PathValue("kitId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.KeycapKitInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		kit := repoapi.KeycapKitToRepo(in)
		kit.KitID = kitID

		updated, err := keycapSetRepo.UpdateKit(r.Context(), setID, kit)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(r.Context()).Warn("keycap set mutation conflict", log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Conflict(w, "the keycap set is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("updating keycap kit", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Internal(w, "failed to update kit")
			return
		}

		out, err := repoapi.KeycapKitToAPI(r.Context(), *updated, images)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping keycap kit to API", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, updated.KitID)
			problem.Internal(w, "failed to update kit")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// DeleteKeycapKit reads the {userId}, {keycapSetId} (parent set), and {kitId} path
// values and requires an authenticated caller. userId must be the caller's
// own subject; deleting a kit from another user's set, or a set that
// doesn't exist, both return 404. Idempotent: a kitId not present in the
// set is not an error.
func DeleteKeycapKit(keycapSetRepo repository.KeycapSetRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		setID := r.PathValue("keycapSetId")
		kitID := r.PathValue("kitId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		err := keycapSetRepo.DeleteKit(r.Context(), setID, kitID)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(r.Context()).Warn("keycap set mutation conflict", log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Conflict(w, "the keycap set is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("deleting keycap kit", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Internal(w, "failed to delete kit")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// setKeycapKitImageResponse is the response body for a successful
// SetKeycapKitImage call. Not a generated api.* type - the OpenAPI spec
// declares this response as an inline object schema, so oapi-codegen
// doesn't emit a named struct for it.
type setKeycapKitImageResponse struct {
	UploadURL string `json:"upload_url"`
}

// SetKeycapKitImage reads the {userId}, {keycapSetId} (parent set), and
// {kitId} path values and requires an authenticated caller. userId must be
// the caller's own subject; setting a kit's image on another user's set, a
// set that doesn't exist, or a kitId that doesn't exist within it, all
// return 404, to avoid revealing it exists. Doesn't upload the image
// itself - the response is a presigned S3 PUT URL the client uploads
// directly to.
func SetKeycapKitImage(keycapSetRepo repository.KeycapSetRepository, lookupRepo repository.LookupRepository, images repository.KeycapKitImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		setID := r.PathValue("keycapSetId")
		kitID := r.PathValue("kitId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.SetKeycapKitImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		fieldErrs, err := repository.ValidateFields(r.Context(), lookupRepo, []repository.FieldCheck{
			{Field: "content_type", Value: in.ContentType, Category: repository.CategoryImageContentType},
		})
		if err != nil {
			log.FromContext(r.Context()).Error("validating keycap kit image content_type", log.Error, err)
			problem.Internal(w, "failed to validate content_type")
			return
		}
		if len(fieldErrs) > 0 {
			problem.ValidationFailed(w, "one or more fields are not approved lookup values", []problem.InvalidParam{
				{Name: "content_type", Reason: fmt.Sprintf("%q is not an approved %s value", in.ContentType, repository.CategoryImageContentType)},
			})
			return
		}

		key := fmt.Sprintf("keycap-sets/%s/%s/kits/%s/image", ownerID, setID, kitID)

		uploadURL, err := images.PresignPut(r.Context(), key, in.ContentType)
		if err != nil {
			log.FromContext(r.Context()).Error("presigning keycap kit image upload", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Internal(w, "failed to set kit image")
			return
		}

		_, err = keycapSetRepo.SetKitImagePath(r.Context(), setID, kitID, key)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(r.Context()).Warn("keycap set mutation conflict", log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Conflict(w, "the keycap set is being modified concurrently, please retry")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("setting keycap kit image path", log.Error, err, log.KeycapSetID, setID, log.KeycapKitID, kitID)
			problem.Internal(w, "failed to set kit image")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(setKeycapKitImageResponse{UploadURL: uploadURL})
	}
}
