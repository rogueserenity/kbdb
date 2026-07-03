package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// echoTool is a no-op tool proving the MCP framework works end to end,
// mirroring /v1/ping's role on the REST side — not real functionality.
var echoTool = mcp.NewTool("echo",
	mcp.WithDescription("Echoes back the provided message."),
	mcp.WithString("message",
		mcp.Description("Message to echo back."),
		mcp.Required(),
	),
)

func handleEcho(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	message, err := req.RequireString("message")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(message), nil
}

func registerTools(s *server.MCPServer) {
	s.AddTool(echoTool, handleEcho)
}
