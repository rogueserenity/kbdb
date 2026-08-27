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
	switchImageStore repository.SwitchImageStore,
	keyboardRepo repository.KeyboardRepository,
	keyboardImageStore repository.KeyboardImageStore,
	keycapSetRepo repository.KeycapSetRepository,
	imageStore repository.KeycapKitImageStore,
	buildRepo repository.BuildRepository,
	buildImageStore repository.BuildImageStore,
	profileRepo repository.ProfileRepository,
	profileImageStore repository.ProfileImageStore,
) {
	sdkmcp.AddTool(s, listLookupsTool, handleListLookups())
	sdkmcp.AddTool(s, getLookupTool, handleGetLookup())
	sdkmcp.AddTool(s, getProfileTool, handleGetProfile(profileRepo))
	sdkmcp.AddTool(s, createProfileTool, handleCreateProfile(profileRepo))
	sdkmcp.AddTool(s, updateProfileTool, handleUpdateProfile(profileRepo))
	sdkmcp.AddTool(s, deleteProfileTool, handleDeleteProfile(profileRepo, profileImageStore))
	sdkmcp.AddTool(s, listSwitchesTool, handleListSwitches(switchRepo))
	sdkmcp.AddTool(s, getSwitchTool, handleGetSwitch(switchRepo))
	sdkmcp.AddTool(s, createSwitchTool, handleCreateSwitch(switchRepo))
	sdkmcp.AddTool(s, updateSwitchTool, handleUpdateSwitch(switchRepo))
	sdkmcp.AddTool(s, deleteSwitchTool, handleDeleteSwitch(switchRepo, buildRepo, buildImageStore, switchImageStore))
	sdkmcp.AddTool(s, setSwitchImageTool, handleSetSwitchImage(switchRepo, switchImageStore))
	sdkmcp.AddTool(s, deleteSwitchImageTool, handleDeleteSwitchImage(switchRepo, switchImageStore))
	sdkmcp.AddTool(s, listKeyboardsTool, handleListKeyboards(keyboardRepo))
	sdkmcp.AddTool(s, getKeyboardTool, handleGetKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, listKeyboardImagesTool, handleListKeyboardImages(keyboardRepo))
	sdkmcp.AddTool(s, createKeyboardTool, handleCreateKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, updateKeyboardTool, handleUpdateKeyboard(keyboardRepo))
	sdkmcp.AddTool(s, deleteKeyboardTool, handleDeleteKeyboard(keyboardRepo, buildRepo, buildImageStore, keyboardImageStore))
	sdkmcp.AddTool(s, addKeyboardImageTool, handleAddKeyboardImage(keyboardRepo, keyboardImageStore))
	sdkmcp.AddTool(s, deleteKeyboardImageTool, handleDeleteKeyboardImage(keyboardRepo, keyboardImageStore))
	sdkmcp.AddTool(s, listKeycapSetsTool, handleListKeycapSets(keycapSetRepo))
	sdkmcp.AddTool(s, getKeycapSetTool, handleGetKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, createKeycapSetTool, handleCreateKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, updateKeycapSetTool, handleUpdateKeycapSet(keycapSetRepo))
	sdkmcp.AddTool(s, deleteKeycapSetTool, handleDeleteKeycapSet(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	sdkmcp.AddTool(s, createKeycapKitTool, handleCreateKeycapKit(keycapSetRepo))
	sdkmcp.AddTool(s, updateKeycapKitTool, handleUpdateKeycapKit(keycapSetRepo))
	sdkmcp.AddTool(s, deleteKeycapKitTool, handleDeleteKeycapKit(keycapSetRepo, buildRepo, buildImageStore, imageStore))
	sdkmcp.AddTool(s, setKeycapKitImageTool, handleSetKeycapKitImage(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, deleteKeycapKitImageTool, handleDeleteKeycapKitImage(keycapSetRepo, imageStore))
	sdkmcp.AddTool(s, createBuildTool, handleCreateBuild(buildRepo, keyboardRepo, switchRepo, keycapSetRepo))
	sdkmcp.AddTool(s, getBuildTool, handleGetBuild(buildRepo))
	sdkmcp.AddTool(s, listBuildsTool, handleListBuilds(buildRepo, keyboardRepo))
	sdkmcp.AddTool(s, updateBuildTool, handleUpdateBuild(buildRepo, keyboardRepo, switchRepo, keycapSetRepo))
	sdkmcp.AddTool(s, deleteBuildTool, handleDeleteBuild(buildRepo, buildImageStore))
	sdkmcp.AddTool(s, addBuildImageTool, handleAddBuildImage(buildRepo, buildImageStore))
	sdkmcp.AddTool(s, deleteBuildImageTool, handleDeleteBuildImage(buildRepo, buildImageStore))
	sdkmcp.AddTool(s, listBuildImagesTool, handleListBuildImages(buildRepo))
}
