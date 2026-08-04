package mcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var listLookupsTool = &mcp.Tool{
	Name:        "list_lookups",
	Description: "Lists every lookup category name (e.g. switch_type, vendor, keyboard_size). Call get_lookup with a category name to see its approved values.",
}

var getLookupTool = &mcp.Tool{
	Name:        "get_lookup",
	Description: "Returns the approved values for one lookup category.",
}

func handleListLookups(repo repository.LookupRepository) mcp.ToolHandlerFor[schema.ListLookupsInput, schema.ListLookupsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ schema.ListLookupsInput) (*mcp.CallToolResult, schema.ListLookupsOutput, error) {
		categories, err := repo.ListCategories(ctx)
		if err != nil {
			log.FromContext(ctx).Error("listing lookup categories", log.Error, err)
			return nil, schema.ListLookupsOutput{}, errors.New("failed to list lookup categories")
		}

		return nil, schema.ListLookupsOutput{Categories: categories}, nil
	}
}

func handleGetLookup(repo repository.LookupRepository) mcp.ToolHandlerFor[schema.GetLookupInput, schema.GetLookupOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetLookupInput) (*mcp.CallToolResult, schema.GetLookupOutput, error) {
		lookup, err := repo.GetCategory(ctx, in.Category)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.GetLookupOutput{}, fmt.Errorf("lookup category %q not found", in.Category)
		}
		if err != nil {
			log.FromContext(ctx).Error("getting lookup category", log.Error, err, log.LookupCategory, in.Category)
			return nil, schema.GetLookupOutput{}, errors.New("failed to get lookup category")
		}

		out, err := repomcp.LookupToMCP(*lookup)
		if err != nil {
			log.FromContext(ctx).Error("mapping lookup category to MCP shape", log.LookupCategory, in.Category, log.Error, err)
			return nil, schema.GetLookupOutput{}, errors.New("failed to get lookup category")
		}

		return nil, out, nil
	}
}
