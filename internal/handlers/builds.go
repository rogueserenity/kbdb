package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

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

// validateBuildLookups writes a 400 listing every invalid field if any check
// fails. An unset (nil) field is skipped, not treated as invalid.
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

// validateBuildReferences writes a 400 listing every invalid reference if
// any check fails (a repository error instead writes a 500). Matches
// validateBuildLookups' signature/shape so CreateBuild can call both the
// same way.
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

// GetBuild reads the {userId} and {buildId} path values. Anonymous callers
// are allowed; a build that exists but isn't readable by the caller returns
// 404, not 403, to avoid revealing it exists.
func GetBuild(repo repository.BuildRepository, images repository.BuildImageStore) http.HandlerFunc {
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

		out, err := repoapi.BuildToAPI(r.Context(), *b, images)
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

// CreateBuild reads the {userId} path value and requires an authenticated
// caller. userId must be the caller's own subject; creating in another
// user's collection returns 404, not 403, to avoid revealing it exists.
func CreateBuild(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
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

		out, err := repoapi.BuildToAPI(r.Context(), *created, images)
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
