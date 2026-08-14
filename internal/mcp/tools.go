package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// registerTools is the single list of every tool this server exposes -
// add a new tool here, not by editing New.
func registerTools(
	s *sdkmcp.Server,
	switchRepo repository.SwitchRepository,
	keyboardRepo repository.KeyboardRepository,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
) {
	sdkmcp.AddTool(s, listLookupsTool, handleListLookups())
	sdkmcp.AddTool(s, getLookupTool, handleGetLookup())
	sdkmcp.AddTool(s, listSwitchesTool, handleListSwitches(switchRepo))
	sdkmcp.AddTool(s, getSwitchTool, handleGetSwitch(switchRepo))
	sdkmcp.AddTool(s, createSwitchTool, handleCreateSwitch(switchRepo))
	sdkmcp.AddTool(s, updateSwitchTool, handleUpdateSwitch(switchRepo))
	sdkmcp.AddTool(s, deleteSwitchTool, handleDeleteSwitch(switchRepo))
	sdkmcp.AddTool(s, listKeyboardsTool, handleListKeyboards(keyboardRepo))
	sdkmcp.AddTool(s, getKeyboardTool, handleGetKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, createKeyboardTool, handleCreateKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, updateKeyboardTool, handleUpdateKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, deleteKeyboardTool, handleDeleteKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, listKeycapSetsTool, handleListKeycapSets(keycapSetRepo))
	sdkmcp.AddTool(s, getKeycapSetTool, handleGetKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, getKeycapKitImageURLTool, handleGetKeycapKitImageURL(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createKeycapSetTool, handleCreateKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, updateKeycapSetTool, handleUpdateKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, deleteKeycapSetTool, handleDeleteKeycapSet(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createKeycapKitTool, handleCreateKeycapKit(keycapSetRepo))
	sdkmcp.AddTool(s, updateKeycapKitTool, handleUpdateKeycapKit(keycapSetRepo))
	sdkmcp.AddTool(s, deleteKeycapKitTool, handleDeleteKeycapKit(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, setKeycapKitImageTool, handleSetKeycapKitImage(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, deleteKeycapKitImageTool, handleDeleteKeycapKitImage(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createBuildTool, handleCreateBuild(buildRepo, keyboardRepo, switchRepo, keycapSetRepo))
	sdkmcp.AddTool(s, getBuildTool, handleGetBuild(buildRepo))
	sdkmcp.AddTool(s, listBuildsTool, handleListBuilds(buildRepo, keyboardRepo))
	sdkmcp.AddTool(s, updateBuildTool, handleUpdateBuild(buildRepo, keyboardRepo, switchRepo, keycapSetRepo))
}
