package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// registerTools is the single list of every tool this server exposes -
// add a new tool here, not by editing New.
func registerTools(s *sdkmcp.Server, lookupRepo repository.LookupRepository) {
	sdkmcp.AddTool(s, pingTool, handlePing)
	sdkmcp.AddTool(s, listLookupsTool, handleListLookups(lookupRepo))
	sdkmcp.AddTool(s, getLookupTool, handleGetLookup(lookupRepo))
}
