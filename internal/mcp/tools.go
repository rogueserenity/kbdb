package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// registerTools is the single list of every tool this server exposes -
// add a new tool here, not by editing New.
func registerTools(
	s *sdkmcp.Server,
	lookupRepo repository.LookupRepository,
	switchRepo repository.SwitchRepository,
	keyboardRepo repository.KeyboardRepository,
) {
	sdkmcp.AddTool(s, listLookupsTool, handleListLookups(lookupRepo))
	sdkmcp.AddTool(s, getLookupTool, handleGetLookup(lookupRepo))
	sdkmcp.AddTool(s, listSwitchesTool, handleListSwitches(switchRepo))
	sdkmcp.AddTool(s, getSwitchTool, handleGetSwitch(switchRepo))
	sdkmcp.AddTool(s, createSwitchTool, handleCreateSwitch(switchRepo, lookupRepo))
	sdkmcp.AddTool(s, updateSwitchTool, handleUpdateSwitch(switchRepo, lookupRepo))
	sdkmcp.AddTool(s, deleteSwitchTool, handleDeleteSwitch(switchRepo))
	sdkmcp.AddTool(s, listKeyboardsTool, handleListKeyboards(keyboardRepo))
	sdkmcp.AddTool(s, getKeyboardTool, handleGetKeyboard(keyboardRepo))
}
