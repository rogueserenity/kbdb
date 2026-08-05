package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	ctxpkg "github.com/rogueserenity/kbdb/internal/ctx"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// REST gets these bounds from api/openapi.yaml's Limit param, applied by
// the request validator before a handler runs. MCP has no equivalent
// validation layer, so the tool handler applies them itself.
const (
	defaultListLimit = 20
	maxListLimit     = 100
)

var listSwitchesTool = &mcp.Tool{
	Name:        "list_switches",
	Description: "Lists switches in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_switch for a single switch's full details. Omit user_id to list your own switches.",
}

var getSwitchTool = &mcp.Tool{
	Name:        "get_switch",
	Description: "Returns the full details of one switch. Omit user_id to read from your own collection.",
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

		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetSwitchOutput{}, err
		}

		sw, err := repo.Get(ctx, ownerID, in.SwitchID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.GetSwitchOutput{}, errSwitchNotFound
		}
		if err != nil {
			log.FromContext(ctx).Error("getting switch", log.SwitchID, in.SwitchID, log.Error, err)
			return nil, schema.GetSwitchOutput{}, errors.New("failed to get switch")
		}

		// Same error as a genuine miss: a switch the caller can't read must
		// not be distinguishable from one that doesn't exist, matching
		// handlers.GetSwitch's 404-not-403.
		if !authz.CanReadVisibility(ctx, ownerID, sw.Visibility) {
			return nil, schema.GetSwitchOutput{}, errSwitchNotFound
		}

		return nil, schema.GetSwitchOutput{Switch: repomcp.SwitchToMCP(*sw)}, nil
	}
}

var errSwitchNotFound = errors.New("switch not found")

// errNoCallerIdentity is unreachable while requireBearerToken gates every
// MCP request (see server.go), but fails closed rather than defaulting to
// an empty owner ID if that wiring ever changes.
var errNoCallerIdentity = errors.New("no caller identity on context")

// resolveOwnerID defaults a blank user_id to the caller's own subject, so
// the common "my switches" case needs no argument.
func resolveOwnerID(ctx context.Context, userID string) (string, error) {
	if id := strings.TrimSpace(userID); id != "" {
		return id, nil
	}

	subject, ok := ctxpkg.UserID(ctx)
	if !ok || subject == "" {
		log.FromContext(ctx).Error("MCP tool ran with no caller identity on context")
		return "", errNoCallerIdentity
	}

	return subject, nil
}

func clampListLimit(limit int) int {
	if limit < 1 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}

	return limit
}
