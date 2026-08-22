package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

func validBuildInput() schema.BuildInput {
	return schema.BuildInput{
		Keyboard:   "kb-1",
		Visibility: "private",
	}
}

type HandleCreateBuildSuite struct {
	suite.Suite

	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockSwitches  *mocks.MockSwitchRepository
	mockKeycaps   *mocks.MockKeycapSetRepository
}

func TestHandleCreateBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleCreateBuildSuite))
}

func (s *HandleCreateBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycaps = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleCreateBuildSuite) handler() mcp.ToolHandlerFor[schema.CreateBuildInput, schema.CreateBuildOutput] {
	return handleCreateBuild(s.mockBuilds, s.mockKeyboards, s.mockSwitches, s.mockKeycaps)
}

// stubOwnedKeyboard arranges keyboardRepo.Get to report "kb-1" as existing
// and owned by the test caller - the default reference-validation outcome
// most tests below want, so the handler under test can proceed to the
// behavior they're actually asserting on.
func (s *HandleCreateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil).
		Maybe()
}

func (s *HandleCreateBuildSuite) TestSucceeds() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().NoError(err)
	s.Equal("kb-1", out.Build.Keyboard)
	s.NotEmpty(out.Build.ID, "create must assign a server-generated id")
}

func (s *HandleCreateBuildSuite) TestBlankKeyboard_ReturnsError() {
	in := validBuildInput()
	in.Keyboard = "   "

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keyboard must not be blank")
}

func (s *HandleCreateBuildSuite) TestInvalidVisibility_ReturnsError() {
	in := validBuildInput()
	in.Visibility = "everyone"

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "visibility")
}

func (s *HandleCreateBuildSuite) TestMalformedBuildDate_ReturnsError() {
	in := validBuildInput()
	badDate := "not-a-date"
	in.BuildDate = &badDate

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "build_date")
}

func (s *HandleCreateBuildSuite) TestNonPositiveSwitchCount_ReturnsError() {
	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 0}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "switches[0].count")
}

func (s *HandleCreateBuildSuite) TestUnapprovedStabsName_ReturnsError() {
	in := validBuildInput()
	in.Stabs = &schema.BuildStabs{Name: strPtrMCP("NotApproved")}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "stabs.name")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestUnapprovedCaseMountType_ReturnsError() {
	in := validBuildInput()
	in.CaseMountType = &schema.BuildCaseMountType{Type: strPtrMCP("NotApproved")}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "case_mount_type.type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleCreateBuildSuite) TestAlreadyExists_ReturnsAlreadyExists() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, repository.ErrAlreadyExists)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorIs(err, errBuildAlreadyExists)
}

func (s *HandleCreateBuildSuite) TestRepositoryError_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		Return(nil, errors.New("create failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "failed to create build")
}

func (s *HandleCreateBuildSuite) TestMissingKeyboard_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "keyboard")
}

func (s *HandleCreateBuildSuite) TestMissingSwitch_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(nil, repository.ErrNotFound)

	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 4}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "switches[0].switch")
}

func (s *HandleCreateBuildSuite) TestMissingKeycapSet_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(nil, repository.ErrNotFound)

	in := validBuildInput()
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keycap_kits[0].keycap_set")
}

func (s *HandleCreateBuildSuite) TestKeycapSetFoundButKitMissing_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{{KitID: "other-kit"}}}, nil)

	in := validBuildInput()
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().ErrorContains(err, "keycap_kits[0].kit")
}

func (s *HandleCreateBuildSuite) TestValidReferences_Succeeds() {
	s.stubOwnedKeyboard()
	s.mockSwitches.EXPECT().
		Get(mock.Anything, mock.Anything, "sw-1").
		Return(&repository.Switch{ID: "sw-1"}, nil)
	s.mockKeycaps.EXPECT().
		Get(mock.Anything, mock.Anything, "ks-1").
		Return(&repository.KeycapSet{ID: "ks-1", Kits: []repository.KeycapKit{{KitID: "kit-1"}}}, nil)
	s.mockBuilds.EXPECT().
		Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	in := validBuildInput()
	in.Switches = []schema.BuildSwitchEntry{{Switch: "sw-1", Count: 4}}
	in.KeycapKits = []schema.BuildKeycapKitEntry{{KeycapSet: "ks-1", Kit: "kit-1"}}

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: in})

	s.Require().NoError(err)
	s.NotEmpty(out.Build.ID)
}

func (s *HandleCreateBuildSuite) TestReferenceCheckRepositoryError_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.CreateBuildInput{BuildInput: validBuildInput()})

	s.Require().ErrorContains(err, "failed to validate build")
}

type HandleUpdateBuildSuite struct {
	suite.Suite

	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
	mockSwitches  *mocks.MockSwitchRepository
	mockKeycaps   *mocks.MockKeycapSetRepository
}

func TestHandleUpdateBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleUpdateBuildSuite))
}

func (s *HandleUpdateBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
	s.mockSwitches = mocks.NewMockSwitchRepository(s.T())
	s.mockKeycaps = mocks.NewMockKeycapSetRepository(s.T())
}

func (s *HandleUpdateBuildSuite) handler() mcp.ToolHandlerFor[schema.UpdateBuildInput, schema.UpdateBuildOutput] {
	return handleUpdateBuild(s.mockBuilds, s.mockKeyboards, s.mockSwitches, s.mockKeycaps)
}

func (s *HandleUpdateBuildSuite) stubOwnedKeyboard() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1"}, nil).
		Maybe()
}

func (s *HandleUpdateBuildSuite) TestSucceeds() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, b repository.Build) (*repository.Build, error) {
			return &b, nil
		})

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().NoError(err)
	s.Equal("b-1", out.Build.ID, "update must target the requested id")
}

func (s *HandleUpdateBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "  ",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleUpdateBuildSuite) TestBlankKeyboard_ReturnsError() {
	in := validBuildInput()
	in.Keyboard = "   "

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{BuildID: "b-1", BuildInput: in})

	s.Require().ErrorContains(err, "keyboard must not be blank")
}

func (s *HandleUpdateBuildSuite) TestMissingKeyboard_ReturnsError() {
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, mock.Anything, "kb-1").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorContains(err, "keyboard")
}

func (s *HandleUpdateBuildSuite) TestNotFound_ReturnsNotFound() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "missing",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleUpdateBuildSuite) TestMutationConflict_ReturnsConflictError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorIs(err, errBuildMutationConflict)
}

func (s *HandleUpdateBuildSuite) TestRepositoryError_ReturnsError() {
	s.stubOwnedKeyboard()
	s.mockBuilds.EXPECT().
		Update(mock.Anything, mock.Anything).
		Return(nil, errors.New("update failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.UpdateBuildInput{
		BuildID:    "b-1",
		BuildInput: validBuildInput(),
	})

	s.Require().ErrorContains(err, "failed to update build")
}

func strPtrMCP(s string) *string { return &s }

type HandleListBuildsSuite struct {
	suite.Suite

	mockBuilds    *mocks.MockBuildRepository
	mockKeyboards *mocks.MockKeyboardRepository
}

func TestHandleListBuildsSuite(t *testing.T) {
	suite.Run(t, new(HandleListBuildsSuite))
}

func (s *HandleListBuildsSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockKeyboards = mocks.NewMockKeyboardRepository(s.T())
}

func (s *HandleListBuildsSuite) handler() mcp.ToolHandlerFor[schema.ListBuildsInput, schema.ListBuildsOutput] {
	return handleListBuilds(s.mockBuilds, s.mockKeyboards)
}

func (s *HandleListBuildsSuite) TestEmpty_ReturnsEmptyList() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 20, "").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Empty(out.Builds)
}

func (s *HandleListBuildsSuite) TestSingleBuild_ResolvableKeyboard_DenormalizesBrandAndName() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 20, "").
		Return([]repository.Build{{UserID: callerID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(&repository.Keyboard{ID: "kb-1", Brand: "Keychron", Name: "Q1"}, nil)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Builds, 1)
	s.Require().NotNil(out.Builds[0].Keyboard)
	s.Equal("Keychron", out.Builds[0].Keyboard.Brand)
	s.Equal("Q1", out.Builds[0].Keyboard.Name)
}

func (s *HandleListBuildsSuite) TestBuildWithKeyboardNotFound_OmitsKeyboard() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 20, "").
		Return([]repository.Build{{UserID: callerID, ID: "build-1", Keyboard: "deleted-kb", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "deleted-kb").
		Return(nil, repository.ErrNotFound)

	handler := s.handler()
	_, out, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().NoError(err)
	s.Require().Len(out.Builds, 1)
	s.Nil(out.Builds[0].Keyboard)
}

func (s *HandleListBuildsSuite) TestKeyboardRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 20, "").
		Return([]repository.Build{{UserID: callerID, ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate}}, "", nil)
	s.mockKeyboards.EXPECT().
		Get(mock.Anything, callerID, "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().ErrorContains(err, "failed to list builds")
}

func (s *HandleListBuildsSuite) TestPassesLimitAndCursor() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 5, "abc").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{Limit: 5, Cursor: "abc"})

	s.Require().NoError(err)
}

func (s *HandleListBuildsSuite) TestOtherUserID_ListsThatUsersCollection() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, otherID, mock.Anything, 20, "").
		Return([]repository.Build{}, "", nil)

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{UserID: otherID})

	s.Require().NoError(err)
}

func (s *HandleListBuildsSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		List(mock.Anything, callerID, mock.Anything, 20, "").
		Return(nil, "", errors.New("query failed"))

	handler := s.handler()
	_, _, err := handler(callerContext(s.T()), nil, schema.ListBuildsInput{})

	s.Require().ErrorContains(err, "failed to list builds")
}

type HandleGetBuildSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
}

func TestHandleGetBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleGetBuildSuite))
}

func (s *HandleGetBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
}

func (s *HandleGetBuildSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{
			ID:         "build-1",
			Keyboard:   "kb-1",
			Visibility: repository.VisibilityPrivate,
		}, nil)

	handler := handleGetBuild(s.mockBuilds)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
	s.Equal("build-1", out.Build.ID)
	s.Equal("kb-1", out.Build.Keyboard)
}

func (s *HandleGetBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := handleGetBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "  "})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleGetBuildSuite) TestNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, mock.Anything, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleGetBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "missing"})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleGetBuildSuite) TestOtherUsersPrivateBuild_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{ID: "build-1", Visibility: repository.VisibilityPrivate}, nil)

	handler := handleGetBuild(s.mockBuilds)
	_, _, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleGetBuildSuite) TestOtherUsersSharedVisibilityBuild_Succeeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, otherID, "build-1").
		Return(&repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityAuthenticated}, nil)

	handler := handleGetBuild(s.mockBuilds)
	_, out, err := handler(callerContext(s.T()), nil, schema.GetBuildInput{BuildID: "build-1", UserID: otherID})

	s.Require().NoError(err)
	s.Equal("build-1", out.Build.ID)
}

type HandleDeleteBuildSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
	mockImages *mocks.MockBuildImageStore
}

func TestHandleDeleteBuildSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteBuildSuite))
}

func (s *HandleDeleteBuildSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteBuildSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().Delete(mock.Anything, "build-1").Return(nil, nil)
	s.mockImages.EXPECT().BestEffortDelete(mock.Anything, []repository.BuildImageKey(nil)).Return()

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildSuite) TestBlankBuildID_ReturnsError() {
	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: ""})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleDeleteBuildSuite) TestNotFound_StillSucceeds() {
	s.mockBuilds.EXPECT().Delete(mock.Anything, "missing").Return(nil, repository.ErrNotFound)
	s.mockImages.EXPECT().BestEffortDelete(mock.Anything, []repository.BuildImageKey(nil)).Return()

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "missing"})

	s.Require().NoError(err, "delete is idempotent: a nonexistent id is not an error")
}

func (s *HandleDeleteBuildSuite) TestImageKeysAreBestEffortCleanedUp() {
	keys := []repository.BuildImageKey{"builds/u/build-1/images/img-1", "builds/u/build-1/images/img-2"}
	s.mockBuilds.EXPECT().Delete(mock.Anything, "build-1").Return(keys, nil)
	s.mockImages.EXPECT().BestEffortDelete(mock.Anything, keys).Return()

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil, errors.New("delete failed"))

	handler := handleDeleteBuild(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildInput{BuildID: "build-1"})

	s.Require().ErrorContains(err, "failed to delete build")
}

type HandleAddBuildImageSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
	mockImages *mocks.MockBuildImageStore
}

func TestHandleAddBuildImageSuite(t *testing.T) {
	suite.Run(t, new(HandleAddBuildImageSuite))
}

func (s *HandleAddBuildImageSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleAddBuildImageSuite) TestSucceeds() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.MatchedBy(func(img repository.BuildImage) bool {
			return img.ImageID != ""
		})).
		Return(&repository.BuildImage{ImageID: "img-1"}, nil)

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, out, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/png",
	})

	s.Require().NoError(err)
	s.NotEmpty(out.ImageID)
	s.Equal("https://example.com/upload", out.UploadURL)
}

func (s *HandleAddBuildImageSuite) TestBlankBuildID_ReturnsError() {
	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     " ",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleAddBuildImageSuite) TestUnapprovedContentType_ReturnsError() {
	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "application/exe",
	})

	s.Require().ErrorContains(err, "content_type")
	s.Require().ErrorContains(err, "not an approved")
}

func (s *HandleAddBuildImageSuite) TestBuildNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "missing").
		Return(nil, repository.ErrNotFound)

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "missing",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleAddBuildImageSuite) TestGetError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(nil, errors.New("get item failed"))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "failed to add build image")
}

func (s *HandleAddBuildImageSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.Anything).
		Return(nil, repository.ErrMutationConflict)

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/png",
	})

	s.Require().ErrorIs(err, errBuildMutationConflict)
}

func (s *HandleAddBuildImageSuite) TestPresignError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("", errors.New("s3: access denied"))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "failed to add build image")
}

func (s *HandleAddBuildImageSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().
		Get(mock.Anything, callerID, "build-1").
		Return(&repository.Build{ID: "build-1"}, nil)
	s.mockImages.EXPECT().
		PresignPutBuildImage(mock.Anything, mock.Anything, "image/png").
		Return("https://example.com/upload", nil)
	s.mockBuilds.EXPECT().
		AddImage(mock.Anything, "build-1", mock.Anything).
		Return(nil, errors.New("put item failed"))

	handler := handleAddBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.AddBuildImageInput{
		BuildID:     "build-1",
		ContentType: "image/png",
	})

	s.Require().ErrorContains(err, "failed to add build image")
}

type HandleDeleteBuildImageSuite struct {
	suite.Suite

	mockBuilds *mocks.MockBuildRepository
	mockImages *mocks.MockBuildImageStore
}

func TestHandleDeleteBuildImageSuite(t *testing.T) {
	suite.Run(t, new(HandleDeleteBuildImageSuite))
}

func (s *HandleDeleteBuildImageSuite) SetupTest() {
	s.mockBuilds = mocks.NewMockBuildRepository(s.T())
	s.mockImages = mocks.NewMockBuildImageStore(s.T())
}

func (s *HandleDeleteBuildImageSuite) TestSucceeds() {
	key := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(&key, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key).Return(nil)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().NoError(err)
}

func (s *HandleDeleteBuildImageSuite) TestBlankBuildID_ReturnsError() {
	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: " ", ImageID: "img-1"})

	s.Require().ErrorContains(err, "build_id must not be blank")
}

func (s *HandleDeleteBuildImageSuite) TestBlankImageID_ReturnsError() {
	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: " "})

	s.Require().ErrorContains(err, "image_id must not be blank")
}

func (s *HandleDeleteBuildImageSuite) TestAlreadyAbsent_StillSucceeds() {
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(nil, nil)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().NoError(err, "deleting an already-absent image is not an error")
}

func (s *HandleDeleteBuildImageSuite) TestBuildNotFound_ReturnsNotFound() {
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "missing", "img-1").Return(nil, repository.ErrNotFound)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "missing", ImageID: "img-1"})

	s.Require().ErrorIs(err, errBuildNotFound)
}

func (s *HandleDeleteBuildImageSuite) TestMutationConflict_ReturnsConflictError() {
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(nil, repository.ErrMutationConflict)

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().ErrorIs(err, errBuildMutationConflict)
}

func (s *HandleDeleteBuildImageSuite) TestRepositoryError_ReturnsError() {
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(nil, errors.New("delete failed"))

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().ErrorContains(err, "failed to delete build image")
}

func (s *HandleDeleteBuildImageSuite) TestImageDeleteFailure_ReturnsError() {
	key := repository.BuildImageKey("builds/u/build-1/images/img-1")
	s.mockBuilds.EXPECT().DeleteImage(mock.Anything, "build-1", "img-1").Return(&key, nil)
	s.mockImages.EXPECT().DeleteBuildImage(mock.Anything, key).Return(errors.New("s3 delete failed"))

	handler := handleDeleteBuildImage(s.mockBuilds, s.mockImages)
	_, _, err := handler(callerContext(s.T()), nil, schema.DeleteBuildImageInput{BuildID: "build-1", ImageID: "img-1"})

	s.Require().ErrorContains(err, "failed to delete build image")
}
