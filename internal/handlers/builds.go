package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/buildrefs"
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/problem"
	"github.com/rogueserenity/kbdb/internal/repoapi"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// An unset (nil) field is skipped, not treated as invalid.
func validateBuildLookups(ctx context.Context, w http.ResponseWriter, b repository.Build) (ok bool) {
	fieldErrs := lookup.ValidateBuild(ctx, b)
	if len(fieldErrs) > 0 {
		invalidParams := make([]problem.InvalidParam, len(fieldErrs))
		for i, fe := range fieldErrs {
			invalidParams[i] = problem.InvalidParam{
				Name:   fe.Field,
				Reason: fmt.Sprintf("%q is not an approved %s value", fe.Value, fe.Category),
			}
		}
		problem.ValidationFailed(w, "one or more fields are not approved lookup values", invalidParams)
		return false
	}

	return true
}

// A repository error writes a 500 rather than a 400.
func validateBuildReferences(
	ctx context.Context, w http.ResponseWriter, ownerID string, b repository.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) (ok bool) {
	fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, keycapSetRepo)
	if err != nil {
		log.FromContext(ctx).Error("validating build references", log.Error, err)
		problem.Internal(w, "failed to validate build")
		return false
	}
	if len(fieldErrs) > 0 {
		invalidParams := make([]problem.InvalidParam, len(fieldErrs))
		for i, fe := range fieldErrs {
			invalidParams[i] = problem.InvalidParam{Name: fe.Field, Reason: fe.Reason}
		}
		problem.ValidationFailed(w, "one or more fields do not reference resources in your collection", invalidParams)
		return false
	}

	return true
}

// ListBuilds handles GET /v1/users/{userId}/builds.
func ListBuilds(repo repository.BuildRepository, keyboardRepo repository.KeyboardRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		limit := parseListLimit(r)
		cursor := r.URL.Query().Get("cursor")

		visibilities := authz.ReadableVisibilities(r.Context(), ownerID)

		builds, nextCursor, err := repo.List(r.Context(), ownerID, visibilities, limit, cursor)
		if err != nil {
			log.FromContext(r.Context()).Error("listing builds", log.Error, err)
			problem.Internal(w, "failed to list builds")
			return
		}

		items := make([]api.BuildSummary, len(builds))
		errs := make([]error, len(builds))

		ctx := r.Context()
		var wg sync.WaitGroup
		for i, b := range builds {
			wg.Add(1)
			go func(i int, b repository.Build) {
				defer wg.Done()

				summary, err := repoapi.BuildToAPISummary(ctx, b, keyboardRepo, images)
				if err != nil {
					errs[i] = fmt.Errorf("mapping build %q to API summary: %w", b.ID, err)
					return
				}
				items[i] = summary
			}(i, b)
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			log.FromContext(r.Context()).Error("mapping builds to API summaries", log.Error, err)
			problem.Internal(w, "failed to list builds")
			return
		}

		page := api.BuildListPage{Items: &items}
		if nextCursor != "" {
			page.NextCursor = &nextCursor
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(page)
	}
}

// GetBuild returns 404, not 403, for a build that exists but isn't
// readable by the caller, to avoid revealing it exists.
func GetBuild(
	repo repository.BuildRepository,
	images repository.BuildImageStore,
	kitImages repository.KeycapKitImageStore,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		b, err := repo.Get(r.Context(), ownerID, id)
		if errors.Is(err, repository.ErrNotFound) {
			problem.NotFound(w, "resource not found")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("getting build", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to get build")
			return
		}

		if !authz.CanReadVisibility(r.Context(), ownerID, b.Visibility) {
			log.DeniedRead(r.Context(), "build", ownerID, string(b.Visibility), log.BuildID, id)
			problem.NotFound(w, "resource not found")
			return
		}

		out, err := repoapi.BuildToAPI(r.Context(), *b, images, kitImages, keyboardRepo, switchRepo, keycapSetRepo, authz.IsOwner(r.Context(), ownerID))
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to get build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// CreateBuild returns 404, not 403, when creating in another user's
// collection, to avoid revealing it exists.
func CreateBuild(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	kitImages repository.KeycapKitImageStore,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.BuildInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		b := repoapi.BuildToRepo(in)

		if !validateBuildLookups(r.Context(), w, b) {
			return
		}

		if !validateBuildReferences(r.Context(), w, ownerID, b, keyboardRepo, switchRepo, keycapSetRepo) {
			return
		}

		b.ID = uuid.NewString()

		created, err := buildRepo.Create(r.Context(), b)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it the same way CreateKeycapSet does.
			problem.Conflict(w, "build already exists")
			return
		}
		if err != nil {
			log.FromContext(r.Context()).Error("creating build", log.Error, err, log.BuildID, b.ID)
			problem.Internal(w, "failed to create build")
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := repoapi.BuildToAPI(r.Context(), *created, images, kitImages, keyboardRepo, switchRepo, keycapSetRepo, true)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, created.ID)
			problem.Internal(w, "failed to create build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// UpdateBuild reads the {userId} and {buildId} path values and requires an
// authenticated caller. userId must be the caller's own subject; updating
// another user's build, or one that doesn't exist, both return 404, to
// avoid revealing it exists.
func UpdateBuild(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	kitImages repository.KeycapKitImageStore,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.BuildInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		b := repoapi.BuildToRepo(in)

		if !validateBuildLookups(r.Context(), w, b) {
			return
		}

		if !validateBuildReferences(r.Context(), w, ownerID, b, keyboardRepo, switchRepo, keycapSetRepo) {
			return
		}

		b.ID = id

		updated, err := buildRepo.Update(r.Context(), b)
		if handleMutationError(w, r, err, log.BuildID, id) {
			return
		}

		// isOwner: true - already gated by authz.IsOwner above.
		out, err := repoapi.BuildToAPI(r.Context(), *updated, images, kitImages, keyboardRepo, switchRepo, keycapSetRepo, true)
		if err != nil {
			log.FromContext(r.Context()).Error("mapping build to API", log.Error, err, log.BuildID, updated.ID)
			problem.Internal(w, "failed to update build")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(out)
	}
}

// AddBuildImage reads the {userId} and {buildId} path values and requires
// an authenticated caller. userId must be the caller's own subject; adding
// an image to another user's build, or one that doesn't exist, both return
// 404. Doesn't upload the image itself - the response is a presigned S3 PUT
// URL the client uploads directly to. The repository mutation (which
// checks existence/ownership) runs before presigning, so a 404 doesn't
// pay for a wasted S3 round trip.
func AddBuildImage(buildRepo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		buildID := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		var in api.CreateBuildImageJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			problem.BadRequest(w, "invalid request body")
			return
		}

		if fieldErr := lookup.ValidateImageContentType(r.Context(), in.ContentType); fieldErr != nil {
			problem.ValidationFailed(w, "one or more fields are not approved lookup values", []problem.InvalidParam{
				{Name: "content_type", Reason: fmt.Sprintf("%q is not an approved %s value", in.ContentType, lookup.CategoryImageContentType)},
			})
			return
		}

		imageID := uuid.NewString()

		key, err := repository.NewBuildImageKey(r.Context(), buildID, imageID)
		if err != nil {
			log.FromContext(r.Context()).Error("building build image key", log.Error, err, log.BuildID, buildID)
			problem.Internal(w, "failed to add build image")
			return
		}

		_, err = buildRepo.AddImage(r.Context(), buildID, repository.BuildImage{ImageID: imageID, Path: key})
		if handleMutationError(w, r, err, log.BuildID, buildID) {
			return
		}

		uploadURL, err := images.PresignPutBuildImage(r.Context(), key, in.ContentType)
		if err != nil {
			log.FromContext(r.Context()).Error("presigning build image upload", log.Error, err, log.BuildID, buildID)
			problem.Internal(w, "failed to add build image")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(api.BuildImageUpload{ImageId: imageID, UploadUrl: uploadURL})
	}
}

// DeleteBuildImage reads the {userId}, {buildId}, and {imageId} path values
// and requires an authenticated caller. userId must be the caller's own
// subject; removing an image from another user's build always returns 404.
// Idempotent: an imageId not present on the build is not an error.
func DeleteBuildImage(buildRepo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		buildID := r.PathValue("buildId")
		imageID := r.PathValue("imageId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		removed, err := buildRepo.DeleteImage(r.Context(), buildID, imageID)
		if handleMutationError(w, r, err, log.BuildID, buildID) {
			return
		}

		if removed != nil {
			if err := images.DeleteBuildImage(r.Context(), *removed); err != nil {
				log.FromContext(r.Context()).Error("deleting build image object", log.Error, err, log.BuildID, buildID)
				problem.Internal(w, "failed to delete build image")
				return
			}
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// DeleteBuild reads the {userId} and {buildId} path values and requires an
// authenticated caller. userId must be the caller's own subject; deleting
// another user's build always returns 404. Deleting is idempotent: a build
// that doesn't exist returns 204.
func DeleteBuild(buildRepo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID := r.PathValue("userId")
		id := r.PathValue("buildId")

		if !authz.IsOwner(r.Context(), ownerID) {
			problem.NotFound(w, "resource not found")
			return
		}

		imageKeys, err := buildRepo.Delete(r.Context(), id)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(r.Context()).Error("deleting build", log.Error, err, log.BuildID, id)
			problem.Internal(w, "failed to delete build")
			return
		}

		images.BestEffortDelete(r.Context(), imageKeys)

		w.WriteHeader(http.StatusNoContent)
	}
}
