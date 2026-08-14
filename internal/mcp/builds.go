package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/buildrefs"
	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errBuildAlreadyExists = errors.New("build already exists")

var errBuildNotFound = errors.New("build not found")

var errBuildMutationConflict = errors.New("the build is being modified concurrently, please retry")

// errNoCaller mirrors [errNoTokenInfo]'s fail-closed shape: unreachable in
// practice since [identityMiddleware] always sets a caller ID before a tool
// handler runs, but this fails closed rather than validating references
// against an empty ownerID if that wiring is ever broken.
var errNoCaller = errors.New("no caller identity on context")

var createBuildTool = &mcp.Tool{
	Name:        "create_build",
	Description: "Adds a build to your own collection, recording an actual keyboard you've assembled from a keyboard, switches, keycap kit(s), and stabilizer/mount/foam details. keyboard must be the id of a Keyboard resource you own, switches[].switch must each be the id of a Switch resource you own, and keycap_kits[].keycap_set/kit must each name a KeycapSet you own and one of its kits - all are verified to exist and belong to you. stabs.name, stabs.mount_type, case_mount_type.type, and case_mount_type.durometer must be approved lookup values - call list_lookups and get_lookup to see them. Images aren't set here - a future tool adds them afterward.",
}

var getBuildTool = &mcp.Tool{
	Name:        "get_build",
	Description: "Returns the full details of one build. Images are reported via has_images rather than URLs - a future tool fetches them on demand. Omit user_id to read from your own collection.",
}

var listBuildsTool = &mcp.Tool{
	Name:        "list_builds",
	Description: "Lists builds in a user's collection, most useful for browsing. Returns an abbreviated shape, including the referenced keyboard's brand/name; call get_build for a single build's full details. Omit user_id to list your own builds.",
}

var updateBuildTool = &mcp.Tool{
	Name:        "update_build",
	Description: "Replaces every field of one of your own builds - fields omitted from the call are cleared, not left unchanged. keyboard must be the id of a Keyboard resource you own, switches[].switch must each be the id of a Switch resource you own, and keycap_kits[].keycap_set/kit must each name a KeycapSet you own and one of its kits - all are verified to exist and belong to you. stabs.name, stabs.mount_type, case_mount_type.type, and case_mount_type.durometer must be approved lookup values - call list_lookups and get_lookup to see them. Images are managed separately and are unaffected by this call.",
}

var deleteBuildTool = &mcp.Tool{
	Name:        "delete_build",
	Description: "Removes a build from your own collection, along with any images. Idempotent: deleting a build that isn't there succeeds.",
}

func handleListBuilds(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
) mcp.ToolHandlerFor[schema.ListBuildsInput, schema.ListBuildsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListBuildsInput) (*mcp.CallToolResult, schema.ListBuildsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListBuildsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		builds, nextCursor, err := buildRepo.List(ctx, ownerID, visibilities, clampListLimit(in.Limit), in.Cursor)
		if err != nil {
			log.FromContext(ctx).Error("listing builds", log.Error, err)
			return nil, schema.ListBuildsOutput{}, errors.New("failed to list builds")
		}

		items := make([]schema.BuildSummary, len(builds))
		for i, b := range builds {
			summary, err := repomcp.BuildToMCPSummary(ctx, b, keyboardRepo)
			if err != nil {
				log.FromContext(ctx).Error("mapping build to MCP summary", log.BuildID, b.ID, log.Error, err)
				return nil, schema.ListBuildsOutput{}, errors.New("failed to list builds")
			}
			items[i] = summary
		}

		return nil, schema.ListBuildsOutput{Builds: items, NextCursor: nextCursor}, nil
	}
}

func handleGetBuild(repo repository.BuildRepository) mcp.ToolHandlerFor[schema.GetBuildInput, schema.GetBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetBuildInput) (*mcp.CallToolResult, schema.GetBuildOutput, error) {
		if strings.TrimSpace(in.BuildID) == "" {
			return nil, schema.GetBuildOutput{}, errors.New("build_id must not be blank")
		}

		b, err := ownedReadable(ctx, repo.Get, func(b repository.Build) repository.Visibility { return b.Visibility },
			"build", errBuildNotFound, log.BuildID, in.UserID, in.BuildID)
		if err != nil {
			return nil, schema.GetBuildOutput{}, err
		}

		return nil, schema.GetBuildOutput{Build: repomcp.BuildToMCP(*b)}, nil
	}
}

func handleCreateBuild(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.CreateBuildInput, schema.CreateBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateBuildInput) (*mcp.CallToolResult, schema.CreateBuildOutput, error) {
		b, err := validatedBuild(ctx, in.BuildInput)
		if err != nil {
			return nil, schema.CreateBuildOutput{}, err
		}

		ownerID, ok := kbdbctx.UserID(ctx)
		if !ok {
			return nil, schema.CreateBuildOutput{}, errNoCaller
		}

		fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, keycapSetRepo)
		if err != nil {
			log.FromContext(ctx).Error("validating build references", log.Error, err)
			return nil, schema.CreateBuildOutput{}, errors.New("failed to validate build")
		}
		if len(fieldErrs) > 0 {
			reasons := make([]string, len(fieldErrs))
			for i, fe := range fieldErrs {
				reasons[i] = fmt.Sprintf("%s: %q %s", fe.Field, fe.Value, fe.Reason)
			}
			return nil, schema.CreateBuildOutput{}, errors.New(strings.Join(reasons, "; "))
		}

		b.ID = uuid.NewString()

		created, err := buildRepo.Create(ctx, b)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateBuildOutput{}, errBuildAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating build", log.BuildID, b.ID, log.Error, err)
			return nil, schema.CreateBuildOutput{}, errors.New("failed to create build")
		}

		return nil, schema.CreateBuildOutput{Build: repomcp.BuildToMCP(*created)}, nil
	}
}

func handleUpdateBuild(
	buildRepo repository.BuildRepository,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.UpdateBuildInput, schema.UpdateBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateBuildInput) (*mcp.CallToolResult, schema.UpdateBuildOutput, error) {
		if strings.TrimSpace(in.BuildID) == "" {
			return nil, schema.UpdateBuildOutput{}, errors.New("build_id must not be blank")
		}

		b, err := validatedBuild(ctx, in.BuildInput)
		if err != nil {
			return nil, schema.UpdateBuildOutput{}, err
		}

		ownerID, ok := kbdbctx.UserID(ctx)
		if !ok {
			return nil, schema.UpdateBuildOutput{}, errNoCaller
		}

		fieldErrs, err := buildrefs.ValidateReferences(ctx, ownerID, b, keyboardRepo, switchRepo, keycapSetRepo)
		if err != nil {
			log.FromContext(ctx).Error("validating build references", log.Error, err)
			return nil, schema.UpdateBuildOutput{}, errors.New("failed to validate build")
		}
		if len(fieldErrs) > 0 {
			reasons := make([]string, len(fieldErrs))
			for i, fe := range fieldErrs {
				reasons[i] = fmt.Sprintf("%s: %q %s", fe.Field, fe.Value, fe.Reason)
			}
			return nil, schema.UpdateBuildOutput{}, errors.New(strings.Join(reasons, "; "))
		}

		b.ID = in.BuildID

		updated, err := buildRepo.Update(ctx, b)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.UpdateBuildOutput{}, errBuildNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			// Warn, not Error: expected contention under retry, not a bug -
			// still worth a trace if one build sees this repeatedly.
			log.FromContext(ctx).Warn("build mutation conflict", log.BuildID, b.ID)
			return nil, schema.UpdateBuildOutput{}, errBuildMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("updating build", log.BuildID, b.ID, log.Error, err)
			return nil, schema.UpdateBuildOutput{}, errors.New("failed to update build")
		}

		return nil, schema.UpdateBuildOutput{Build: repomcp.BuildToMCP(*updated)}, nil
	}
}

// validatedBuild checks in code what api/openapi.yaml declares for REST: the
// SDK infers tool schemas from Go types alone, so there is no per-field
// constraint to attach.
func validatedBuild(ctx context.Context, in schema.BuildInput) (repository.Build, error) {
	if strings.TrimSpace(in.Keyboard) == "" {
		return repository.Build{}, errors.New("keyboard must not be blank")
	}

	if in.BuildDate != nil {
		if _, err := time.Parse(dateLayout, *in.BuildDate); err != nil {
			return repository.Build{}, fmt.Errorf("build_date: %q must be a date in YYYY-MM-DD form", *in.BuildDate)
		}
	}

	for i, entry := range in.Switches {
		if entry.Count < 1 {
			return repository.Build{}, fmt.Errorf("switches[%d].count: %d must be at least 1", i, entry.Count)
		}
	}

	b := repomcp.BuildFromMCP(in)

	if !b.Visibility.Valid() {
		return repository.Build{}, fmt.Errorf(
			"visibility %q must be one of: public, authenticated, private", in.Visibility)
	}

	fieldErrs := lookup.ValidateBuild(ctx, b)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
		}

		return repository.Build{}, errors.New(strings.Join(reasons, "; "))
	}

	return b, nil
}

func handleDeleteBuild(
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
) mcp.ToolHandlerFor[schema.DeleteBuildInput, schema.DeleteBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteBuildInput) (*mcp.CallToolResult, schema.DeleteBuildOutput, error) {
		if strings.TrimSpace(in.BuildID) == "" {
			return nil, schema.DeleteBuildOutput{}, errors.New("build_id must not be blank")
		}

		imageKeys, err := buildRepo.Delete(ctx, in.BuildID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(ctx).Error("deleting build", log.BuildID, in.BuildID, log.Error, err)
			return nil, schema.DeleteBuildOutput{}, errors.New("failed to delete build")
		}

		images.BestEffortDelete(ctx, imageKeys)

		return nil, schema.DeleteBuildOutput{}, nil
	}
}
