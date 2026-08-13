package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errBuildAlreadyExists = errors.New("build already exists")

var createBuildTool = &mcp.Tool{
	Name:        "create_build",
	Description: "Adds a build to your own collection, recording an actual keyboard you've assembled from a keyboard, switches, keycap kit(s), and stabilizer/mount/foam details. keyboard must be the id of a Keyboard resource - this is not currently verified to exist or belong to you, so double-check the id before calling. stabs.name, stabs.mount_type, case_mount_type.type, and case_mount_type.durometer must be approved lookup values - call list_lookups and get_lookup to see them. Images aren't set here - a future tool adds them afterward.",
}

func handleCreateBuild(
	buildRepo repository.BuildRepository,
) mcp.ToolHandlerFor[schema.CreateBuildInput, schema.CreateBuildOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateBuildInput) (*mcp.CallToolResult, schema.CreateBuildOutput, error) {
		b, err := validatedBuild(ctx, in.BuildInput)
		if err != nil {
			return nil, schema.CreateBuildOutput{}, err
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
