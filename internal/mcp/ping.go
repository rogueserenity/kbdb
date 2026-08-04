package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/repository"
)

var pingTool = &mcp.Tool{
	Name:        "ping",
	Description: "Responds with a trivial OK to confirm the request reached real tool logic.",
}

func handlePing(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "ok"}},
	}, nil, nil
}

func registerTools(s *mcp.Server, lookupRepo repository.LookupRepository) {
	mcp.AddTool(s, pingTool, handlePing)
	mcp.AddTool(s, listLookupsTool, handleListLookups(lookupRepo))
	mcp.AddTool(s, getLookupTool, handleGetLookup(lookupRepo))
}
