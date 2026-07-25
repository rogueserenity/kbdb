package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// pingTool is a no-op tool proving the MCP framework works end to end.
var pingTool = mcp.NewTool("ping",
	mcp.WithDescription("Responds with a trivial OK to confirm the request reached real tool logic."),
)

func handlePing(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText("ok"), nil
}

func registerTools(s *server.MCPServer) {
	s.AddTool(pingTool, handlePing)
}
