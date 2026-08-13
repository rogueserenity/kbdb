package repomcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type BuildToMCPSuite struct {
	suite.Suite
}

func TestBuildToMCPSuite(t *testing.T) {
	suite.Run(t, new(BuildToMCPSuite))
}

func strPtr(s string) *string     { return &s }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }

func (s *BuildToMCPSuite) TestMapsAllFields() {
	plate := "Brass"
	notes := "first build"
	buildDate := "2026-01-15"

	out := BuildToMCP(repository.Build{
		ID:       "build-1",
		Keyboard: "kb-1",
		Plate:    &plate,
		CaseMountType: &repository.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &repository.BuildStabs{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: []repository.BuildSwitchEntry{
			{Switch: "sw-1", Count: 70},
		},
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks-1", Kit: "kit-1"},
		},
		BuildDate:  &buildDate,
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
		Images: []repository.BuildImage{
			{ImageID: "img-1", Path: repository.BuildImageKey("builds/u-1/build-1/images/img-1")},
		},
	})

	s.Equal("build-1", out.ID)
	s.Equal("kb-1", out.Keyboard)
	s.Equal(&plate, out.Plate)
	s.Require().NotNil(out.CaseMountType)
	s.Equal("Top Mount", *out.CaseMountType.Type)
	s.Equal("70A", *out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal("Durock v3", *out.Stabs.Name)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].Switch)
	s.Equal(70, out.Switches[0].Count)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks-1", out.KeycapKits[0].KeycapSet)
	s.Equal("kit-1", out.KeycapKits[0].Kit)
	s.Equal(&buildDate, out.BuildDate)
	s.Equal("public", out.Visibility)
	s.True(out.HasImages)
}

func (s *BuildToMCPSuite) TestNoImages_HasImagesFalse() {
	out := BuildToMCP(repository.Build{ID: "build-1", Keyboard: "kb-1", Visibility: repository.VisibilityPrivate})

	s.False(out.HasImages)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
}

func (s *BuildToMCPSuite) TestBuildFromMCP_MapsAllFields() {
	plate := "Brass"
	notes := "first build"
	buildDate := "2026-01-15"

	out := BuildFromMCP(schema.BuildInput{
		Keyboard: "kb-1",
		Plate:    &plate,
		CaseMountType: &schema.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &schema.BuildStabs{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: []schema.BuildSwitchEntry{
			{Switch: "sw-1", Count: 70},
		},
		KeycapKits: []schema.BuildKeycapKitEntry{
			{KeycapSet: "ks-1", Kit: "kit-1"},
		},
		BuildDate:  &buildDate,
		Notes:      &notes,
		Visibility: "public",
	})

	s.Equal("kb-1", out.Keyboard)
	s.Equal(&plate, out.Plate)
	s.Require().NotNil(out.CaseMountType)
	s.Equal("Top Mount", *out.CaseMountType.Type)
	s.Require().NotNil(out.Stabs)
	s.Equal("Durock v3", *out.Stabs.Name)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw-1", out.Switches[0].Switch)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks-1", out.KeycapKits[0].KeycapSet)
	s.Equal(&buildDate, out.BuildDate)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Empty(out.ID, "ID is the caller's responsibility, not this mapping's")
	s.Nil(out.Images, "a build write never carries images")
}

func (s *BuildToMCPSuite) TestBuildFromMCP_AllOptionalFieldsNil_MapsToNil() {
	out := BuildFromMCP(schema.BuildInput{Keyboard: "kb-1", Visibility: "private"})

	s.Nil(out.Plate)
	s.Nil(out.CaseMountType)
	s.Nil(out.Stabs)
	s.Nil(out.Foam)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
	s.Nil(out.BuildDate)
	s.Nil(out.Notes)
}

type BuildToMCPSummarySuite struct {
	suite.Suite
}

func TestBuildToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(BuildToMCPSummarySuite))
}

func (s *BuildToMCPSummarySuite) TestResolvableKeyboard_DenormalizesBrandAndName() {
	buildDate := "2026-01-15"
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1", BuildDate: &buildDate}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := BuildToMCPSummary(context.Background(), b, keyboards)
	s.Require().NoError(err)

	s.Equal("build-1", out.ID)
	s.Equal(&buildDate, out.BuildDate)
	s.False(out.HasImage)
	s.Require().NotNil(out.Keyboard)
	s.Equal("Keychron", out.Keyboard.Brand)
	s.Equal("Q1", out.Keyboard.Name)
}

func (s *BuildToMCPSummarySuite) TestKeyboardNotFound_OmitsKeyboardRatherThanFailing() {
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1"}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(nil, repository.ErrNotFound)

	out, err := BuildToMCPSummary(context.Background(), b, keyboards)
	s.Require().NoError(err)

	s.Nil(out.Keyboard)
}

func (s *BuildToMCPSummarySuite) TestKeyboardRepositoryError_ReturnsError() {
	b := repository.Build{UserID: "alice", ID: "build-1", Keyboard: "kb-1"}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := BuildToMCPSummary(context.Background(), b, keyboards)
	s.Require().Error(err)
}

func (s *BuildToMCPSummarySuite) TestHasImages_ReportsTrue() {
	b := repository.Build{
		UserID: "alice", ID: "build-1", Keyboard: "kb-1",
		Images: []repository.BuildImage{{ImageID: "img-1"}},
	}

	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb-1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb-1"}, nil)

	out, err := BuildToMCPSummary(context.Background(), b, keyboards)
	s.Require().NoError(err)

	s.True(out.HasImage)
}
