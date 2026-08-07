package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errSwitchNotFound = errors.New("switch not found")

var errSwitchAlreadyExists = errors.New("switch already exists")

var listSwitchesTool = &mcp.Tool{
	Name:        "list_switches",
	Description: "Lists switches in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_switch for a single switch's full details. Omit user_id to list your own switches.",
}

var getSwitchTool = &mcp.Tool{
	Name:        "get_switch",
	Description: "Returns the full details of one switch. Omit user_id to read from your own collection.",
}

var createSwitchTool = &mcp.Tool{
	Name:        "create_switch",
	Description: "Adds a switch to your own collection. type, material, spring.material, and purchase.vendor must be approved lookup values - call list_lookups and get_lookup to see them.",
}

var updateSwitchTool = &mcp.Tool{
	Name:        "update_switch",
	Description: "Replaces a switch in your own collection. Every field is replaced, so omitting an optional field clears it; send the full switch, not just the fields you want to change.",
}

var deleteSwitchTool = &mcp.Tool{
	Name:        "delete_switch",
	Description: "Removes a switch from your own collection. Idempotent: deleting a switch that isn't there succeeds.",
}

func handleListSwitches(repo repository.SwitchRepository) mcp.ToolHandlerFor[schema.ListSwitchesInput, schema.ListSwitchesOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListSwitchesInput) (*mcp.CallToolResult, schema.ListSwitchesOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListSwitchesOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		switches, nextCursor, err := repo.List(ctx, ownerID, visibilities, clampListLimit(in.Limit), in.Cursor)
		if err != nil {
			log.FromContext(ctx).Error("listing switches", log.Error, err)
			return nil, schema.ListSwitchesOutput{}, errors.New("failed to list switches")
		}

		items := make([]schema.SwitchSummary, len(switches))
		for i, sw := range switches {
			items[i] = repomcp.SwitchToMCPSummary(sw)
		}

		return nil, schema.ListSwitchesOutput{Switches: items, NextCursor: nextCursor}, nil
	}
}

func handleGetSwitch(repo repository.SwitchRepository) mcp.ToolHandlerFor[schema.GetSwitchInput, schema.GetSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetSwitchInput) (*mcp.CallToolResult, schema.GetSwitchOutput, error) {
		if strings.TrimSpace(in.SwitchID) == "" {
			return nil, schema.GetSwitchOutput{}, errors.New("switch_id must not be blank")
		}

		sw, err := ownedReadable(ctx, repo.Get, func(sw repository.Switch) repository.Visibility { return sw.Visibility },
			"switch", errSwitchNotFound, log.SwitchID, in.UserID, in.SwitchID)
		if err != nil {
			return nil, schema.GetSwitchOutput{}, err
		}

		return nil, schema.GetSwitchOutput{Switch: repomcp.SwitchToMCP(*sw)}, nil
	}
}

func handleCreateSwitch(
	switchRepo repository.SwitchRepository,
	lookupRepo repository.LookupRepository,
) mcp.ToolHandlerFor[schema.CreateSwitchInput, schema.CreateSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateSwitchInput) (*mcp.CallToolResult, schema.CreateSwitchOutput, error) {
		sw, err := validatedSwitch(ctx, lookupRepo, in.SwitchInput)
		if err != nil {
			return nil, schema.CreateSwitchOutput{}, err
		}

		sw.ID = uuid.NewString()

		created, err := switchRepo.Create(ctx, sw)
		if errors.Is(err, repository.ErrAlreadyExists) {
			// Practically unreachable - ID is a fresh UUID, not caller
			// input - but Create's ConditionExpression guards a collision
			// regardless, so surface it rather than reporting an internal
			// failure, matching handlers.CreateSwitch's 409.
			return nil, schema.CreateSwitchOutput{}, errSwitchAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating switch", log.SwitchID, sw.ID, log.Error, err)
			return nil, schema.CreateSwitchOutput{}, errors.New("failed to create switch")
		}

		return nil, schema.CreateSwitchOutput{Switch: repomcp.SwitchToMCP(*created)}, nil
	}
}

func handleUpdateSwitch(
	switchRepo repository.SwitchRepository,
	lookupRepo repository.LookupRepository,
) mcp.ToolHandlerFor[schema.UpdateSwitchInput, schema.UpdateSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateSwitchInput) (*mcp.CallToolResult, schema.UpdateSwitchOutput, error) {
		if strings.TrimSpace(in.SwitchID) == "" {
			return nil, schema.UpdateSwitchOutput{}, errors.New("switch_id must not be blank")
		}

		sw, err := validatedSwitch(ctx, lookupRepo, in.SwitchInput)
		if err != nil {
			return nil, schema.UpdateSwitchOutput{}, err
		}

		sw.ID = in.SwitchID

		updated, err := switchRepo.Update(ctx, sw)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.UpdateSwitchOutput{}, errSwitchNotFound
		}
		if err != nil {
			log.FromContext(ctx).Error("updating switch", log.SwitchID, sw.ID, log.Error, err)
			return nil, schema.UpdateSwitchOutput{}, errors.New("failed to update switch")
		}

		return nil, schema.UpdateSwitchOutput{Switch: repomcp.SwitchToMCP(*updated)}, nil
	}
}

func handleDeleteSwitch(switchRepo repository.SwitchRepository) mcp.ToolHandlerFor[schema.DeleteSwitchInput, schema.DeleteSwitchOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteSwitchInput) (*mcp.CallToolResult, schema.DeleteSwitchOutput, error) {
		if strings.TrimSpace(in.SwitchID) == "" {
			return nil, schema.DeleteSwitchOutput{}, errors.New("switch_id must not be blank")
		}

		if err := switchRepo.Delete(ctx, in.SwitchID); err != nil {
			log.FromContext(ctx).Error("deleting switch", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.DeleteSwitchOutput{}, errors.New("failed to delete switch")
		}

		return nil, schema.DeleteSwitchOutput{}, nil
	}
}

// validatedSwitch checks in code what api/openapi.yaml declares for REST:
// the SDK infers tool schemas from Go types alone, so there is no per-field
// constraint to attach.
func validatedSwitch(
	ctx context.Context,
	lookupRepo repository.LookupRepository,
	in schema.SwitchInput,
) (repository.Switch, error) {
	return validatedWrite(ctx, lookupRepo, in, in.Brand, in.Name, in.Visibility, nil,
		repomcp.SwitchFromMCP,
		func(sw *repository.Switch) *repository.Visibility { return &sw.Visibility },
		lookup.ValidateSwitch, defaultFieldErrorReason)
}
