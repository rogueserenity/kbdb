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

// buildToAPIDeps bundles the mocks BuildToAPI needs; the returned
// EXPECT()s must be set up by the caller before invoking BuildToAPI.
type buildToAPIDeps struct {
	images        *mocks.MockBuildImageStore
	kitImages     *mocks.MockKeycapKitImageStore
	keyboardRepo  *mocks.MockKeyboardRepository
	switchRepo    *mocks.MockSwitchRepository
	keycapSetRepo *mocks.MockKeycapSetRepository
}

func newBuildToAPIDeps(t interface {
	mock.TestingT
	Cleanup(func())
}) buildToAPIDeps {
	return buildToAPIDeps{
		images:        mocks.NewMockBuildImageStore(t),
		kitImages:     mocks.NewMockKeycapKitImageStore(t),
		keyboardRepo:  mocks.NewMockKeyboardRepository(t),
		switchRepo:    mocks.NewMockSwitchRepository(t),
		keycapSetRepo: mocks.NewMockKeycapSetRepository(t),
	}
}

func (d buildToAPIDeps) call(ctx context.Context, b repository.Build) (api.Build, error) {
	return BuildToAPI(ctx, b, d.images, d.kitImages, d.keyboardRepo, d.switchRepo, d.keycapSetRepo, true)
}

// expectFullyResolvable sets up every dependency in fullRepoBuild() (kb1,
// sw1, ks1/kit1) to resolve successfully.
func (d buildToAPIDeps) expectFullyResolvable() {
	d.keyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1", Size: strPtr("75%"), Layout: strPtr("ANSI")}, nil)
	d.switchRepo.EXPECT().
		Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().
		Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Profile: strPtr("Cherry"),
			Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base"}},
		}, nil)
}

func (s *BuildToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	b := fullRepoBuild()
	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Equal(b.ID, out.Id)
	s.Require().NotNil(out.Keyboard)
	s.Equal("kb1", out.Keyboard.Id)
	s.Equal("Keychron", out.Keyboard.Brand)
	s.Equal("Q1", out.Keyboard.Name)
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
	s.Require().NotNil((*out.Switches)[0].Switch)
	s.Equal("sw1", (*out.Switches)[0].Switch.Id)
	s.Equal("Oil King", (*out.Switches)[0].Switch.Name)
	s.Equal(70, (*out.Switches)[0].Count)
	s.Require().NotNil(out.KeycapKits)
	s.Require().Len(*out.KeycapKits, 1)
	s.Require().NotNil((*out.KeycapKits)[0].KeycapSet)
	s.Equal("ks1", (*out.KeycapKits)[0].KeycapSet.Id)
	s.Equal("kit1", (*out.KeycapKits)[0].KitId)
	s.Require().NotNil((*out.KeycapKits)[0].KitName)
	s.Equal("Base", *(*out.KeycapKits)[0].KitName)
	s.Require().NotNil(out.BuildDate)
	s.Equal(*b.BuildDate, out.BuildDate.Format(dateLayout))
	s.Equal(b.Notes, out.Notes)
	s.Equal(api.Visibility(b.Visibility), out.Visibility)
}

func (s *BuildToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	b := repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := d.call(context.Background(), b)
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

	d := newBuildToAPIDeps(s.T())
	_, err := d.call(context.Background(), b)

	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestImagesPopulated_MintsFreshPresignedURLPerImage() {
	b := fullRepoBuild()
	b.Images = []repository.BuildImage{
		{ImageID: "img1", Path: repository.BuildImageKey("builds/alice/build1/images/img1")},
		{ImageID: "img2", Path: repository.BuildImageKey("builds/alice/build1/images/img2")},
	}

	d := newBuildToAPIDeps(s.T())
	d.expectFullyResolvable()
	d.images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[0].Path).Return("https://example.com/img1", nil)
	d.images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[1].Path).Return("https://example.com/img2", nil)

	out, err := d.call(context.Background(), b)
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

	d := newBuildToAPIDeps(s.T())
	d.images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[0].Path).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)

	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKeyboardNotFound_OmitsKeyboardRatherThanFailing() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(nil, repository.ErrNotFound)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base"}}}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.Keyboard)
}

func (s *BuildToAPISuite) TestKeyboardRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestSwitchNotFound_KeepsCountOmitsSwitch() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(nil, repository.ErrNotFound)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base"}}}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 1)
	s.Nil((*out.Switches)[0].Switch)
	s.Equal(70, (*out.Switches)[0].Count)
}

func (s *BuildToAPISuite) TestMultipleSwitchEntries_ResolvesEachIndependently() {
	b := fullRepoBuild()
	b.Switches = []repository.BuildSwitchEntry{
		{Switch: "sw-missing", Count: 1},
		{Switch: "sw1", Count: 70},
	}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw-missing").Return(nil, repository.ErrNotFound)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base"}}}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.Switches)
	s.Require().Len(*out.Switches, 2)
	s.Nil((*out.Switches)[0].Switch)
	s.Equal(1, (*out.Switches)[0].Count)
	s.Require().NotNil((*out.Switches)[1].Switch)
	s.Equal("sw1", (*out.Switches)[1].Switch.Id)
	s.Equal(70, (*out.Switches)[1].Count)
}

func (s *BuildToAPISuite) TestSwitchRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKeycapSetNotFound_KeepsKitIDOmitsRest() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(nil, repository.ErrNotFound)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.KeycapKits)
	s.Require().Len(*out.KeycapKits, 1)
	s.Equal("kit1", (*out.KeycapKits)[0].KitId)
	s.Nil((*out.KeycapKits)[0].KeycapSet)
	s.Nil((*out.KeycapKits)[0].KitName)
	s.Nil((*out.KeycapKits)[0].KitImageUrl)
}

func (s *BuildToAPISuite) TestKeycapSetRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").Return(nil, errors.New("dynamo unavailable"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestKitNotFoundInResolvedKeycapSet_KeepsKitIDOmitsRest() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia", Kits: []repository.KeycapKit{{KitID: "other-kit", Name: "Base"}}}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.KeycapKits)
	s.Require().Len(*out.KeycapKits, 1)
	s.Equal("kit1", (*out.KeycapKits)[0].KitId)
	s.Nil((*out.KeycapKits)[0].KeycapSet)
	s.Nil((*out.KeycapKits)[0].KitName)
}

func (s *BuildToAPISuite) TestKitWithImage_MintsFreshPresignedURL() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	imgPath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base", ImagePath: &imgPath}},
		}, nil)
	d.kitImages.EXPECT().PresignGet(mock.Anything, imgPath).Return("https://example.com/kit1.png", nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.KeycapKits)
	s.Require().Len(*out.KeycapKits, 1)
	s.Require().NotNil((*out.KeycapKits)[0].KitImageUrl)
	s.Equal("https://example.com/kit1.png", *(*out.KeycapKits)[0].KitImageUrl)
}

func (s *BuildToAPISuite) TestKitImagePresignFails_ReturnsError() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear"}, nil)
	imgPath := repository.KeycapKitImageKey("keycap-sets/alice/ks1/kits/kit1/image")
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base", ImagePath: &imgPath}},
		}, nil)
	d.kitImages.EXPECT().PresignGet(mock.Anything, imgPath).Return("", errors.New("s3: access denied"))

	_, err := d.call(context.Background(), b)
	s.Require().Error(err)
}

func (s *BuildToAPISuite) TestTotalCost_SumsKeyboardSwitchesKitsAndStabs() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchase: repository.SwitchPurchase{Price: floatPtr(0.5)},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	// fullRepoBuild: 70 switches at 0.5 each = 35; stabs price 12.5.
	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+35+150+12.5, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestTotalCost_UnknownComponentsExcludedNotZeroed() {
	b := fullRepoBuild()
	b.Stabs = nil

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchase: repository.SwitchPurchase{Price: floatPtr(0.5)},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{KitID: "kit1", Name: "Base"}},
		}, nil)

	// Keyboard has no purchase price, keycap kit has no purchase price, and
	// Stabs is nil - only the switches' 70*0.5 = 35 should be counted.
	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Require().NotNil(out.TotalCost)
	s.InDelta(35, *out.TotalCost, 0.0001)
}

func (s *BuildToAPISuite) TestTotalCost_NoPricedComponents_OmitsField() {
	b := repository.Build{UserID: "alice", ID: "build1", Keyboard: "kb1", Visibility: repository.VisibilityPrivate}

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := d.call(context.Background(), b)
	s.Require().NoError(err)

	s.Nil(out.TotalCost)
}

func (s *BuildToAPISuite) TestIsOwnerFalse_OmitsStabsPriceAndTotalCost() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchase: repository.SwitchPurchase{Price: floatPtr(0.5)},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := BuildToAPI(context.Background(), b, d.images, d.kitImages, d.keyboardRepo, d.switchRepo, d.keycapSetRepo, false)
	s.Require().NoError(err)

	s.Require().NotNil(out.Stabs)
	s.Nil(out.Stabs.Price)
	s.Equal(b.Stabs.Name, out.Stabs.Name)
	s.Equal(b.Stabs.MountType, out.Stabs.MountType)
	s.Nil(out.TotalCost)
}

func (s *BuildToAPISuite) TestIsOwnerTrue_IncludesStabsPriceAndTotalCost() {
	b := fullRepoBuild()

	d := newBuildToAPIDeps(s.T())
	d.keyboardRepo.EXPECT().Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{
			UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1",
			Purchase: repository.KeyboardPurchase{Price: floatPtr(200)},
		}, nil)
	d.switchRepo.EXPECT().Get(mock.Anything, "alice", "sw1").
		Return(&repository.Switch{
			UserID: "alice", ID: "sw1", Brand: "Gateron", Name: "Oil King", Type: "Linear",
			Purchase: repository.SwitchPurchase{Price: floatPtr(0.5)},
		}, nil)
	d.keycapSetRepo.EXPECT().Get(mock.Anything, "alice", "ks1").
		Return(&repository.KeycapSet{
			UserID: "alice", ID: "ks1", Brand: "GMK", Name: "Olivia",
			Kits: []repository.KeycapKit{{
				KitID: "kit1", Name: "Base",
				Purchase: repository.KeycapKitPurchase{Price: floatPtr(150)},
			}},
		}, nil)

	out, err := BuildToAPI(context.Background(), b, d.images, d.kitImages, d.keyboardRepo, d.switchRepo, d.keycapSetRepo, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Stabs)
	s.Equal(b.Stabs.Price, out.Stabs.Price)
	s.Require().NotNil(out.TotalCost)
	s.InDelta(200+35+150+12.5, *out.TotalCost, 0.0001)
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

type BuildToAPISummarySuite struct {
	suite.Suite
}

func TestBuildToAPISummarySuite(t *testing.T) {
	suite.Run(t, new(BuildToAPISummarySuite))
}

func (s *BuildToAPISummarySuite) TestResolvableKeyboard_DenormalizesBrandAndName() {
	b := fullRepoBuild()

	images := mocks.NewMockBuildImageStore(s.T())
	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().NoError(err)

	s.Equal(&b.ID, out.Id)
	s.Require().NotNil(out.Keyboard)
	s.Require().NotNil(out.Keyboard.Brand)
	s.Equal("Keychron", *out.Keyboard.Brand)
	s.Require().NotNil(out.Keyboard.Name)
	s.Equal("Q1", *out.Keyboard.Name)
}

func (s *BuildToAPISummarySuite) TestKeyboardNotFound_OmitsKeyboardRatherThanFailing() {
	b := fullRepoBuild()

	images := mocks.NewMockBuildImageStore(s.T())
	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, repository.ErrNotFound)

	out, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().NoError(err)

	s.Nil(out.Keyboard)
}

func (s *BuildToAPISummarySuite) TestKeyboardRepositoryError_ReturnsError() {
	b := fullRepoBuild()

	images := mocks.NewMockBuildImageStore(s.T())
	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(nil, errors.New("dynamo unavailable"))

	_, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().Error(err)
}

func (s *BuildToAPISummarySuite) TestNoImages_ImageNil() {
	b := fullRepoBuild()

	images := mocks.NewMockBuildImageStore(s.T())
	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().NoError(err)

	s.Nil(out.Image)
}

func (s *BuildToAPISummarySuite) TestImagesPresent_UsesFirstImageOnly() {
	b := fullRepoBuild()
	b.Images = []repository.BuildImage{
		{ImageID: "img1", Path: repository.BuildImageKey("builds/alice/build1/images/img1")},
		{ImageID: "img2", Path: repository.BuildImageKey("builds/alice/build1/images/img2")},
	}

	images := mocks.NewMockBuildImageStore(s.T())
	images.EXPECT().PresignGetBuildImage(mock.Anything, b.Images[0].Path).Return("https://example.com/img1", nil)
	keyboards := mocks.NewMockKeyboardRepository(s.T())
	keyboards.EXPECT().
		Get(mock.Anything, "alice", "kb1").
		Return(&repository.Keyboard{UserID: "alice", ID: "kb1", Brand: "Keychron", Name: "Q1"}, nil)

	out, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().NoError(err)

	s.Require().NotNil(out.Image)
	s.Equal("img1", out.Image.ImageId)
	s.Equal("https://example.com/img1", out.Image.Url)
}

func (s *BuildToAPISummarySuite) TestMalformedBuildDate_ReturnsError() {
	b := fullRepoBuild()
	b.BuildDate = strPtr("not-a-date")

	images := mocks.NewMockBuildImageStore(s.T())
	keyboards := mocks.NewMockKeyboardRepository(s.T())

	_, err := BuildToAPISummary(context.Background(), b, keyboards, images)
	s.Require().Error(err)
}
