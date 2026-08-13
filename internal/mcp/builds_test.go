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

func strPtrMCP(s string) *string { return &s }
