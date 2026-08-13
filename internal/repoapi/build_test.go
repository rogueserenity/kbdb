package repoapi

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

func fullRepoBuild() repository.Build {
	return repository.Build{
		UserID:   "alice",
		ID:       "build1",
		Keyboard: "kb1",
		Plate:    strPtr("Brass"),
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
			{Switch: "sw1", Count: 70},
		},
		KeycapKits: []repository.BuildKeycapKitEntry{
			{KeycapSet: "ks1", Kit: "kit1"},
		},
		BuildDate:  strPtr("2026-01-15"),
		Notes:      strPtr("first build"),
		Visibility: repository.VisibilityPrivate,
	}
}

type BuildToAPISuite struct {
	suite.Suite
}

func TestBuildToAPISuite(t *testing.T) {
	suite.Run(t, new(BuildToAPISuite))
}

func (s *BuildToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	b := fullRepoBuild()
	images := mocks.NewMockBuildImageStore(s.T())

	out, err := BuildToAPI(context.Background(), b, images)
	s.Require().NoError(err)

	s.Equal(b.ID, out.Id)
	s.Equal(b.Keyboard, out.Keyboard)
	s.Equal(b.Plate, out.Plate)
	s.Require().NotNil(out.CaseMountType)
	s.Equal(b.CaseMountType.Type, out.CaseMountType.Type)
	s.Equal(b.CaseMountType.Durometer, out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal(b.Stabs.Name, out.Stabs.Name)
	s.Equal(b.Stabs.MountType, out.Stabs.MountType)
	s.Equal(b.Stabs.Price, out.Stabs.Price)
	s.Equal(b.Foam, out.Foam)
	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 1)
	s.Equal("sw1", (*out.Switches)[0].Switch)
	s.Equal(70, (*out.Switches)[0].Count)
	s.Require().NotNil(out.KeycapKits)
	s.Require().Len(*out.KeycapKits, 1)
	s.Equal("ks1", (*out.KeycapKits)[0].KeycapSet)
	s.Equal("kit1", (*out.KeycapKits)[0].Kit)
	s.Require().NotNil(out.BuildDate)
	s.Equal(*b.BuildDate, out.BuildDate.Format(dateLayout))
	s.Equal(b.Notes, out.Notes)
	s.Equal(api.Visibility(b.Visibility), out.Visibility)
}

func (s *BuildToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	b := repository.Build{ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate}

	images := mocks.NewMockBuildImageStore(s.T())
	out, err := BuildToAPI(context.Background(), b, images)
	s.Require().NoError(err)

	s.Nil(out.Plate)
	s.Nil(out.CaseMountType)
	s.Nil(out.Stabs)
	s.Nil(out.Foam)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
	s.Nil(out.BuildDate)
	s.Nil(out.Notes)
	s.Nil(out.Images)
}

func (s *BuildToAPISuite) TestMalformedStoredBuildDate_ReturnsError() {
	b := fullRepoBuild()
	b.BuildDate = strPtr("not-a-date")

	images := mocks.NewMockBuildImageStore(s.T())
	_, err := BuildToAPI(context.Background(), b, images)

	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestImagesPopulated_MintsFreshPresignedURLPerImage() {
	b := fullRepoBuild()
	b.Images = []repository.BuildImage{
		{ImageID: "img1", Path: repository.BuildImageKey("builds/alice/build1/images/img1")},
		{ImageID: "img2", Path: repository.BuildImageKey("builds/alice/build1/images/img2")},
	}

	images := mocks.NewMockBuildImageStore(s.T())
	images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[0].Path).Return("https://example.com/img1", nil)
	images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[1].Path).Return("https://example.com/img2", nil)

	out, err := BuildToAPI(context.Background(), b, images)
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Require().Len(*out.Images, 2)
	s.Equal("img1", (*out.Images)[0].ImageId)
	s.Equal("https://example.com/img1", (*out.Images)[0].Url)
	s.Equal("img2", (*out.Images)[1].ImageId)
	s.Equal("https://example.com/img2", (*out.Images)[1].Url)
}

func (s *BuildToAPISuite) TestPresignFails_ReturnsError() {
	b := fullRepoBuild()
	b.Images = []repository.BuildImage{
		{ImageID: "img1", Path: repository.BuildImageKey("builds/alice/build1/images/img1")},
	}

	images := mocks.NewMockBuildImageStore(s.T())
	images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[0].Path).Return("", errors.New("s3: access denied"))

	_, err := BuildToAPI(context.Background(), b, images)

	s.Require().Error(err)
}

func fullAPIBuildInput() api.BuildInput {
	return api.BuildInput{
		Keyboard: "kb1",
		Plate:    strPtr("Brass"),
		CaseMountType: &api.BuildCaseMountType{
			Type:      strPtr("Top Mount"),
			Durometer: strPtr("70A"),
		},
		Stabs: &api.BuildStabs{
			Name:      strPtr("Durock v3"),
			MountType: strPtr("Screw-in"),
			Price:     floatPtr(12.5),
		},
		Foam: boolPtr(true),
		Switches: &[]api.BuildSwitchEntry{
			{Switch: "sw1", Count: 70},
		},
		KeycapKits: &[]api.BuildKeycapKitEntry{
			{KeycapSet: "ks1", Kit: "kit1"},
		},
		Notes:      strPtr("first build"),
		Visibility: api.Visibility(repository.VisibilityPrivate),
	}
}

type BuildToRepoSuite struct {
	suite.Suite
}

func TestBuildToRepoSuite(t *testing.T) {
	suite.Run(t, new(BuildToRepoSuite))
}

func (s *BuildToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIBuildInput()
	out := BuildToRepo(in)

	s.Equal(in.Keyboard, out.Keyboard)
	s.Equal(in.Plate, out.Plate)
	s.Require().NotNil(out.CaseMountType)
	s.Equal(in.CaseMountType.Type, out.CaseMountType.Type)
	s.Equal(in.CaseMountType.Durometer, out.CaseMountType.Durometer)
	s.Require().NotNil(out.Stabs)
	s.Equal(in.Stabs.Name, out.Stabs.Name)
	s.Equal(in.Stabs.MountType, out.Stabs.MountType)
	s.Equal(in.Stabs.Price, out.Stabs.Price)
	s.Equal(in.Foam, out.Foam)
	s.Require().Len(out.Switches, 1)
	s.Equal("sw1", out.Switches[0].Switch)
	s.Equal(70, out.Switches[0].Count)
	s.Require().Len(out.KeycapKits, 1)
	s.Equal("ks1", out.KeycapKits[0].KeycapSet)
	s.Equal("kit1", out.KeycapKits[0].Kit)
	s.Equal(in.Notes, out.Notes)
	s.Equal(repository.Visibility(in.Visibility), out.Visibility)
	s.Empty(out.UserID)
	s.Empty(out.ID)
	s.Nil(out.Images, "a build write never carries images")
}

func (s *BuildToRepoSuite) TestAllOptionalFieldsNil_MapsToNil() {
	in := api.BuildInput{Keyboard: "kb1", Visibility: api.Visibility(repository.VisibilityPrivate)}

	out := BuildToRepo(in)

	s.Nil(out.Plate)
	s.Nil(out.CaseMountType)
	s.Nil(out.Stabs)
	s.Nil(out.Foam)
	s.Nil(out.Switches)
	s.Nil(out.KeycapKits)
	s.Nil(out.BuildDate)
	s.Nil(out.Notes)
}
