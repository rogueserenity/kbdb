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

func fullRepoKeycapSet() repository.KeycapSet {
	return repository.KeycapSet{
		UserID:     "alice",
		ID:         "ks1",
		Brand:      "GMK",
		Name:       "Laser",
		Profile:    strPtr("Cherry"),
		Material:   strPtr("ABS"),
		Notes:      strPtr("group buy"),
		Visibility: repository.VisibilityPrivate,
	}
}

type KeycapSetToAPISuite struct {
	suite.Suite
}

func TestKeycapSetToAPISuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToAPISuite))
}

func (s *KeycapSetToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	ks := fullRepoKeycapSet()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	out, err := KeycapSetToAPI(context.Background(), ks, images, true)
	s.Require().NoError(err)

	s.Equal(ks.ID, out.Id)
	s.Equal(ks.Brand, out.Brand)
	s.Equal(ks.Name, out.Name)
	s.Equal(ks.Profile, out.Profile)
	s.Equal(ks.Material, out.Material)
	s.Equal(ks.Notes, out.Notes)
	s.Equal(api.Visibility(ks.Visibility), out.Visibility)
}

func (s *KeycapSetToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	ks := repository.KeycapSet{ID: "ks1", Brand: "GMK", Name: "Laser", Visibility: repository.VisibilityPrivate}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	out, err := KeycapSetToAPI(context.Background(), ks, images, true)
	s.Require().NoError(err)

	s.Nil(out.Profile)
	s.Nil(out.Material)
	s.Nil(out.Notes)
	s.Nil(out.Kits)
}

func (s *KeycapSetToAPISuite) TestKitsPopulated_MapsEachKit() {
	ks := fullRepoKeycapSet()
	ks.Kits = []repository.KeycapKit{
		{KitID: "kit1", Name: "Base"},
		{KitID: "kit2", Name: "Extension"},
	}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	out, err := KeycapSetToAPI(context.Background(), ks, images, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 2)
	s.Equal("kit1", (*out.Kits)[0].KitId)
	s.Equal("Base", (*out.Kits)[0].Name)
	s.Equal("kit2", (*out.Kits)[1].KitId)
	s.Equal("Extension", (*out.Kits)[1].Name)
}

func (s *KeycapSetToAPISuite) TestMalformedStoredKitPurchaseDate_ReturnsError() {
	ks := fullRepoKeycapSet()
	ks.Kits = []repository.KeycapKit{
		{KitID: "kit1", Name: "Base", Purchase: repository.KeycapKitPurchase{OrderDate: strPtr("not-a-date")}},
	}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	_, err := KeycapSetToAPI(context.Background(), ks, images, true)

	s.Require().Error(err)
}

func (s *KeycapSetToAPISuite) TestIsOwnerFalse_OmitsKitPriceKeepsRestOfPurchase() {
	ks := fullRepoKeycapSet()
	ks.Kits = []repository.KeycapKit{fullRepoKeycapKit()}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *ks.Kits[0].ImagePath).Return("https://example.com/presigned-get", nil)

	out, err := KeycapSetToAPI(context.Background(), ks, images, false)
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 1)
	kit := (*out.Kits)[0]
	s.Require().NotNil(kit.Purchase)
	s.Nil(kit.Purchase.Price)
	s.Equal(ks.Kits[0].Purchase.Vendor, kit.Purchase.Vendor)
	s.Equal(ks.Kits[0].Purchase.OrderStatus, kit.Purchase.OrderStatus)
}

func (s *KeycapSetToAPISuite) TestIsOwnerTrue_IncludesKitPrice() {
	ks := fullRepoKeycapSet()
	ks.Kits = []repository.KeycapKit{fullRepoKeycapKit()}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *ks.Kits[0].ImagePath).Return("https://example.com/presigned-get", nil)

	out, err := KeycapSetToAPI(context.Background(), ks, images, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Kits)
	s.Require().Len(*out.Kits, 1)
	s.Require().NotNil((*out.Kits)[0].Purchase)
	s.Equal(ks.Kits[0].Purchase.Price, (*out.Kits)[0].Purchase.Price)
}

func (s *KeycapSetToAPISuite) TestKeycapSetToAPISummary_MapsOnlySummaryFields() {
	ks := fullRepoKeycapSet()

	images := mocks.NewMockKeycapKitImageStore(s.T())

	summary, err := KeycapSetToAPISummary(context.Background(), ks, images)
	s.Require().NoError(err)

	s.Equal(&ks.ID, summary.Id)
	s.Equal(&ks.Brand, summary.Brand)
	s.Equal(&ks.Name, summary.Name)
	s.Equal(ks.Profile, summary.Profile)
	s.Nil(summary.PrimaryKitImage)
}

func (s *KeycapSetToAPISuite) TestKeycapSetToAPISummary_PrimaryKitWithImage_ResolvesImage() {
	ks := fullRepoKeycapSet()
	kit := fullRepoKeycapKit()
	ks.Kits = []repository.KeycapKit{kit}
	ks.PrimaryKitID = &kit.KitID

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *kit.ImagePath).Return("https://example.com/presigned-get", nil)

	summary, err := KeycapSetToAPISummary(context.Background(), ks, images)
	s.Require().NoError(err)

	s.Require().NotNil(summary.PrimaryKitImage)
	s.Equal("https://example.com/presigned-get", summary.PrimaryKitImage.Url)
}

func (s *KeycapSetToAPISuite) TestKeycapSetToAPISummary_PrimaryKitWithNoImage_NilImage() {
	ks := fullRepoKeycapSet()
	kit := fullRepoKeycapKit()
	kit.ImagePath = nil
	ks.Kits = []repository.KeycapKit{kit}
	ks.PrimaryKitID = &kit.KitID

	images := mocks.NewMockKeycapKitImageStore(s.T())

	summary, err := KeycapSetToAPISummary(context.Background(), ks, images)
	s.Require().NoError(err)
	s.Nil(summary.PrimaryKitImage)
}

func (s *KeycapSetToAPISuite) TestKeycapSetToAPISummary_PrimaryKitDeleted_NilImage() {
	ks := fullRepoKeycapSet()
	danglingID := "no-longer-a-kit"
	ks.PrimaryKitID = &danglingID

	images := mocks.NewMockKeycapKitImageStore(s.T())

	summary, err := KeycapSetToAPISummary(context.Background(), ks, images)
	s.Require().NoError(err)
	s.Nil(summary.PrimaryKitImage)
}

func fullAPIKeycapSetInput() api.KeycapSetInput {
	return api.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Laser",
		Profile:    strPtr("Cherry"),
		Material:   strPtr("ABS"),
		Notes:      strPtr("group buy"),
		Visibility: api.Visibility(repository.VisibilityPrivate),
	}
}

type KeycapSetToRepoSuite struct {
	suite.Suite
}

func TestKeycapSetToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToRepoSuite))
}

func (s *KeycapSetToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIKeycapSetInput()
	out := KeycapSetToRepo(in)

	s.Equal(in.Brand, out.Brand)
	s.Equal(in.Name, out.Name)
	s.Equal(in.Profile, out.Profile)
	s.Equal(in.Material, out.Material)
	s.Equal(in.Notes, out.Notes)
	s.Equal(repository.Visibility(in.Visibility), out.Visibility)
	s.Empty(out.UserID)
	s.Empty(out.ID)
}

func (s *KeycapSetToRepoSuite) TestAllOptionalFieldsNil_MapsToNil() {
	in := api.KeycapSetInput{Brand: "GMK", Name: "Laser", Visibility: api.Visibility(repository.VisibilityPrivate)}

	out := KeycapSetToRepo(in)

	s.Nil(out.Profile)
	s.Nil(out.Material)
	s.Nil(out.Notes)
}

func fullRepoKeycapKit() repository.KeycapKit {
	return repository.KeycapKit{
		KitID:     "kit1",
		Name:      "Base",
		ImagePath: imageKeyPtr("keycap-sets/alice/ks1/kits/kit1/image"),
		Purchase: repository.KeycapKitPurchase{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(120.00),
			OrderDate:    strPtr("2026-01-15"),
			DeliveryDate: strPtr("2026-03-01"),
			OrderStatus:  strPtr("delivered"),
		},
	}
}

type KeycapKitToAPISuite struct {
	suite.Suite
}

func TestKeycapKitToAPISuite(t *testing.T) {
	suite.Run(t, new(KeycapKitToAPISuite))
}

func (s *KeycapKitToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *k.ImagePath).Return("https://example.com/presigned-get", nil)

	out, err := KeycapKitToAPI(context.Background(), k, images, true)
	s.Require().NoError(err)

	s.Equal(k.KitID, out.KitId)
	s.Equal(k.Name, out.Name)
	s.Require().NotNil(out.Image)
	s.Equal("https://example.com/presigned-get", out.Image.Url)
	s.Require().NotNil(out.Purchase)
	s.Equal(k.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(k.Purchase.Price, out.Purchase.Price)
	s.Equal(k.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*k.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*k.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
}

func (s *KeycapKitToAPISuite) TestAllOptionalFieldsNil_OmittedNotZeroValue() {
	k := repository.KeycapKit{KitID: "kit1", Name: "Base"}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	out, err := KeycapKitToAPI(context.Background(), k, images, true)
	s.Require().NoError(err)

	s.Nil(out.Purchase)
	s.Nil(out.Image, "no ImagePath set, so images is never called")
}

func (s *KeycapKitToAPISuite) TestMalformedStoredDate_ReturnsError() {
	k := repository.KeycapKit{
		KitID: "kit1", Name: "Base",
		Purchase: repository.KeycapKitPurchase{OrderDate: strPtr("not-a-date")},
	}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	_, err := KeycapKitToAPI(context.Background(), k, images, true)

	s.Require().Error(err)
}

func (s *KeycapKitToAPISuite) TestIsOwnerFalse_OmitsPriceKeepsRestOfPurchase() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *k.ImagePath).Return("https://example.com/presigned-get", nil)

	out, err := KeycapKitToAPI(context.Background(), k, images, false)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(k.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(k.Purchase.OrderStatus, out.Purchase.OrderStatus)
}

func (s *KeycapKitToAPISuite) TestIsOwnerTrue_IncludesPrice() {
	k := fullRepoKeycapKit()
	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *k.ImagePath).Return("https://example.com/presigned-get", nil)

	out, err := KeycapKitToAPI(context.Background(), k, images, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(k.Purchase.Price, out.Purchase.Price)
}

func (s *KeycapKitToAPISuite) TestPresignGetFails_ReturnsError() {
	k := repository.KeycapKit{KitID: "kit1", Name: "Base", ImagePath: imageKeyPtr("keycap-sets/alice/ks1/kits/kit1/image")}

	images := mocks.NewMockKeycapKitImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *k.ImagePath).Return("", errors.New("s3: access denied"))

	_, err := KeycapKitToAPI(context.Background(), k, images, true)

	s.Require().Error(err)
}

func fullAPIKeycapKitInput() api.KeycapKitInput {
	return api.KeycapKitInput{
		Name: "Base",
		Purchase: &api.Purchase{
			Vendor:      strPtr("CannonKeys"),
			Price:       floatPtr(120.00),
			OrderStatus: strPtr("delivered"),
		},
	}
}

type KeycapKitToRepoSuite struct {
	suite.Suite
}

func TestKeycapKitToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeycapKitToRepoSuite))
}

func (s *KeycapKitToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	in := fullAPIKeycapKitInput()
	out := KeycapKitToRepo(in)

	s.Equal(in.Name, out.Name)
	s.Equal(in.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(in.Purchase.Price, out.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Empty(out.KitID)
}

func (s *KeycapKitToRepoSuite) TestPurchaseNil_MapsToZeroValue() {
	in := api.KeycapKitInput{Name: "Base"}

	out := KeycapKitToRepo(in)

	s.Equal(repository.KeycapKitPurchase{}, out.Purchase)
}
