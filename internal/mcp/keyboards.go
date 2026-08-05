package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errKeyboardNotFound = errors.New("keyboard not found")

var listKeyboardsTool = &mcp.Tool{
	Name:        "list_keyboards",
	Description: "Lists keyboards in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_keyboard for a single keyboard's full details. Omit user_id to list your own keyboards.",
}

var getKeyboardTool = &mcp.Tool{
	Name:        "get_keyboard",
	Description: "Returns the full details of one keyboard, including its case/plate design, PCB and purchase history. Omit user_id to read from your own collection.",
}

func handleListKeyboards(repo repository.KeyboardRepository) mcp.ToolHandlerFor[schema.ListKeyboardsInput, schema.ListKeyboardsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeyboardsInput) (*mcp.CallToolResult, schema.ListKeyboardsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListKeyboardsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		keyboards, nextCursor, err := repo.List(ctx, ownerID, visibilities, clampListLimit(in.Limit), in.Cursor)
		if err != nil {
			log.FromContext(ctx).Error("listing keyboards", log.Error, err)
			return nil, schema.ListKeyboardsOutput{}, errors.New("failed to list keyboards")
		}

		items := make([]schema.KeyboardSummary, len(keyboards))
		for i, kb := range keyboards {
			items[i] = repomcp.KeyboardToMCPSummary(kb)
		}

		return nil, schema.ListKeyboardsOutput{Keyboards: items, NextCursor: nextCursor}, nil
	}
}

func handleGetKeyboard(repo repository.KeyboardRepository) mcp.ToolHandlerFor[schema.GetKeyboardInput, schema.GetKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeyboardInput) (*mcp.CallToolResult, schema.GetKeyboardOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.GetKeyboardOutput{}, errors.New("keyboard_id must not be blank")
		}

		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		kb, err := repo.Get(ctx, ownerID, in.KeyboardID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.GetKeyboardOutput{}, errKeyboardNotFound
		}
		if err != nil {
			log.FromContext(ctx).Error("getting keyboard", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.GetKeyboardOutput{}, errors.New("failed to get keyboard")
		}

		// Same error as a genuine miss: a keyboard the caller can't read must
		// not be distinguishable from one that doesn't exist, matching
		// handlers.GetKeyboard's 404-not-403.
		if !authz.CanReadVisibility(ctx, ownerID, kb.Visibility) {
			return nil, schema.GetKeyboardOutput{}, errKeyboardNotFound
		}

		return nil, schema.GetKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*kb)}, nil
	}
}
