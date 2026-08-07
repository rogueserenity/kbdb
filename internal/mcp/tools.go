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
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
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
	sdkmcp.AddTool(s, createKeyboardTool, handleCreateKeyboard(keyboardRepo, lookupRepo))
	sdkmcp.AddTool(s, updateKeyboardTool, handleUpdateKeyboard(keyboardRepo, lookupRepo))
	sdkmcp.AddTool(s, deleteKeyboardTool, handleDeleteKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, listKeycapSetsTool, handleListKeycapSets(keycapSetRepo))
	sdkmcp.AddTool(s, getKeycapSetTool, handleGetKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, getKeycapKitImageURLTool, handleGetKeycapKitImageURL(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createKeycapSetTool, handleCreateKeycapSet(keycapSetRepo, lookupRepo))
	sdkmcp.AddTool(s, updateKeycapSetTool, handleUpdateKeycapSet(keycapSetRepo, lookupRepo))
	sdkmcp.AddTool(s, deleteKeycapSetTool, handleDeleteKeycapSet(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createKeycapKitTool, handleCreateKeycapKit(keycapSetRepo, lookupRepo))
	sdkmcp.AddTool(s, updateKeycapKitTool, handleUpdateKeycapKit(keycapSetRepo, lookupRepo))
	sdkmcp.AddTool(s, deleteKeycapKitTool, handleDeleteKeycapKit(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, setKeycapKitImageTool, handleSetKeycapKitImage(keycapSetRepo, lookupRepo, imageStore))
	sdkmcp.AddTool(s, deleteKeycapKitImageTool, handleDeleteKeycapKitImage(keycapSetRepo, imageStore))
}
