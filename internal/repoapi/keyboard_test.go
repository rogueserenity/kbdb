package repoapi

import (
	"errors"
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

func fullRepoKeyboard() repository.Keyboard {
	return repository.Keyboard{
		UserID: "alice",
		ID:     "kb1",
		Brand:  "Keychron",
		Name:   "Q1",
		Size:   strPtr("75%"),
		Layout: strPtr("WK"),
		Design: repository.KeyboardDesign{
			TopCase:    repository.KeyboardMaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			BottomCase: repository.KeyboardMaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			Weight:     repository.KeyboardMaterialColor{Material: strPtr("Brass"), Color: strPtr("Gold")},
			Plates:     []string{"FR4", "PC"},
		},
		PCB: repository.KeyboardPCB{
			Thickness:    floatPtr(1.6),
			Firmware:     strPtr("QMK/VIA"),
			Assembly:     strPtr("Hot-swap"),
			Connectivity: strPtr("Wired"),
		},
		Purchase: repository.KeyboardPurchase{
			Vendor:       strPtr("Keychron"),
			Price:        floatPtr(199.99),
			OrderDate:    strPtr("2026-01-15"),
			DeliveryDate: strPtr("2026-01-22"),
			OrderStatus:  strPtr("Delivered"),
		},
		Notes:      strPtr("stock lubed"),
		Visibility: repository.VisibilityPrivate,
	}
}

type KeyboardToAPISuite struct {
	suite.Suite
}

func TestKeyboardToAPISuite(t *testing.T) {
	suite.Run(t, new(KeyboardToAPISuite))
}

func (s *KeyboardToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	kb := fullRepoKeyboard()
	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Equal(kb.ID, out.Id)
	s.Equal(kb.Brand, out.Brand)
	s.Equal(kb.Name, out.Name)
	s.Equal(kb.Size, out.Size)
	s.Equal(kb.Layout, out.Layout)
	s.Equal(kb.Notes, out.Notes)
	s.Equal(api.Visibility(kb.Visibility), out.Visibility)

	if s.NotNil(out.Design) {
		s.Require().NotNil(out.Design.TopCase)
		s.Equal(kb.Design.TopCase.Material, out.Design.TopCase.Material)
		s.Equal(kb.Design.TopCase.Color, out.Design.TopCase.Color)
		s.Require().NotNil(out.Design.BottomCase)
		s.Equal(kb.Design.BottomCase.Material, out.Design.BottomCase.Material)
		s.Require().NotNil(out.Design.Weight)
		s.Equal(kb.Design.Weight.Material, out.Design.Weight.Material)
		s.Require().NotNil(out.Design.Plates)
		s.Equal(kb.Design.Plates, *out.Design.Plates)
	}
	if s.NotNil(out.Pcb) {
		s.Equal(kb.PCB.Thickness, out.Pcb.Thickness)
		s.Equal(kb.PCB.Firmware, out.Pcb.Firmware)
		s.Equal(kb.PCB.Assembly, out.Pcb.Assembly)
		s.Equal(kb.PCB.Connectivity, out.Pcb.Connectivity)
	}
	if s.NotNil(out.Purchase) {
		s.Equal(kb.Purchase.Vendor, out.Purchase.Vendor)
		s.Equal(kb.Purchase.Price, out.Purchase.Price)
		s.Equal(kb.Purchase.OrderStatus, out.Purchase.OrderStatus)
		s.Require().NotNil(out.Purchase.OrderDate)
		s.Equal(*kb.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
		s.Require().NotNil(out.Purchase.DeliveryDate)
		s.Equal(*kb.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	}
}

func (s *KeyboardToAPISuite) TestAllOptionalFieldsNil_SubStructsOmitted() {
	kb := repository.Keyboard{ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate}

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Nil(out.Size)
	s.Nil(out.Layout)
	s.Nil(out.Notes)
	s.Nil(out.Design, "an all-nil KeyboardDesign must map to a nil pointer, not an empty object")
	s.Nil(out.Pcb, "an all-nil KeyboardPCB must map to a nil pointer, not an empty object")
	s.Nil(out.Purchase, "an all-nil KeyboardPurchase must map to a nil pointer, not an empty object")
}

func (s *KeyboardToAPISuite) TestOneFieldSetInSubStruct_SubStructPresent() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Design: repository.KeyboardDesign{TopCase: repository.KeyboardMaterialColor{Material: strPtr("Aluminum")}},
	}

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	if s.NotNil(out.Design) {
		s.Require().NotNil(out.Design.TopCase)
		s.Equal(strPtr("Aluminum"), out.Design.TopCase.Material)
		s.Nil(out.Design.BottomCase)
		s.Nil(out.Design.Weight)
		s.Nil(out.Design.Plates)
	}
}

func (s *KeyboardToAPISuite) TestPlatesNil_OmittedFromDesign() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Design: repository.KeyboardDesign{TopCase: repository.KeyboardMaterialColor{Material: strPtr("Aluminum")}},
	}

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Design)
	s.Nil(out.Design.Plates, "a nil Plates slice must map to a nil pointer, not an empty slice")
}

func (s *KeyboardToAPISuite) TestPlatesEmptySlice_PresentNotNil() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Design: repository.KeyboardDesign{Plates: []string{}},
	}

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Design)
	if s.NotNil(out.Design.Plates, "an empty (non-nil) Plates slice must still map to a non-nil pointer") {
		s.Empty(*out.Design.Plates)
	}
}

func (s *KeyboardToAPISuite) TestMalformedStoredDate_ReturnsError() {
	kb := repository.Keyboard{
		ID: "kb1", Brand: "Keychron", Name: "Q1", Visibility: repository.VisibilityPrivate,
		Purchase: repository.KeyboardPurchase{OrderDate: strPtr("not-a-date")},
	}

	_, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)

	s.Require().Error(err)
}

func (s *KeyboardToAPISuite) TestIsOwnerFalse_OmitsPriceKeepsRestOfPurchase() {
	kb := fullRepoKeyboard()

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), false)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(kb.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(kb.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*kb.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*kb.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
}

func (s *KeyboardToAPISuite) TestIsOwnerTrue_IncludesPrice() {
	kb := fullRepoKeyboard()

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(kb.Purchase.Price, out.Purchase.Price)
}

func (s *KeyboardToAPISuite) TestImagesPresent_PresignsEachAndPreservesOrder() {
	kb := fullRepoKeyboard()
	kb.Images = []repository.KeyboardImage{
		{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"},
		{ImageID: "img2", Path: "keyboards/alice/kb1/images/img2"},
	}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().PresignGetKeyboardImage(mock.Anything, kb.Images[0].Path).Return("https://example.com/img1", nil)
	images.EXPECT().PresignGetKeyboardImage(mock.Anything, kb.Images[1].Path).Return("https://example.com/img2", nil)

	out, err := KeyboardToAPI(s.T().Context(), kb, images, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Images)
	s.Require().Len(*out.Images, 2)
	s.Equal("img1", (*out.Images)[0].ImageId)
	s.Equal("https://example.com/img1", (*out.Images)[0].Url)
	s.Equal("img2", (*out.Images)[1].ImageId)
	s.Equal("https://example.com/img2", (*out.Images)[1].Url)
}

func (s *KeyboardToAPISuite) TestNoImages_ImagesFieldNil() {
	kb := fullRepoKeyboard()

	out, err := KeyboardToAPI(s.T().Context(), kb, mocks.NewMockKeyboardImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Nil(out.Images)
}

func (s *KeyboardToAPISuite) TestImagePresignError_Propagates() {
	kb := fullRepoKeyboard()
	kb.Images = []repository.KeyboardImage{{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"}}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().PresignGetKeyboardImage(mock.Anything, kb.Images[0].Path).Return("", errors.New("s3: access denied"))

	_, err := KeyboardToAPI(s.T().Context(), kb, images, true)

	s.Require().Error(err)
}

func (s *KeyboardToAPISuite) TestKeyboardToAPISummary_MapsOnlySummaryFields() {
	kb := fullRepoKeyboard()
	images := mocks.NewMockKeyboardImageStore(s.T())

	summary, err := KeyboardToAPISummary(s.T().Context(), kb, images)
	s.Require().NoError(err)

	s.Equal(&kb.ID, summary.Id)
	s.Equal(&kb.Brand, summary.Brand)
	s.Equal(&kb.Name, summary.Name)
	s.Equal(kb.Size, summary.Size)
	s.Equal(kb.Layout, summary.Layout)
	s.Equal(kb.Purchase.OrderStatus, summary.OrderStatus)
	s.Nil(summary.Image, "no images on the keyboard must map to a nil Image")
}

func (s *KeyboardToAPISuite) TestKeyboardToAPISummary_ImagesPresent_ReturnsFirstImagePresigned() {
	kb := fullRepoKeyboard()
	kb.Images = []repository.KeyboardImage{
		{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"},
		{ImageID: "img2", Path: "keyboards/alice/kb1/images/img2"},
	}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().PresignGetKeyboardImage(mock.Anything, kb.Images[0].Path).Return("https://example.com/img1", nil)

	summary, err := KeyboardToAPISummary(s.T().Context(), kb, images)
	s.Require().NoError(err)

	s.Require().NotNil(summary.Image)
	s.Equal("img1", summary.Image.ImageId)
	s.Equal("https://example.com/img1", summary.Image.Url)
}

func (s *KeyboardToAPISuite) TestKeyboardToAPISummary_PresignError_Propagates() {
	kb := fullRepoKeyboard()
	kb.Images = []repository.KeyboardImage{{ImageID: "img1", Path: "keyboards/alice/kb1/images/img1"}}
	images := mocks.NewMockKeyboardImageStore(s.T())
	images.EXPECT().PresignGetKeyboardImage(mock.Anything, kb.Images[0].Path).Return("", errors.New("s3: access denied"))

	_, err := KeyboardToAPISummary(s.T().Context(), kb, images)

	s.Require().Error(err)
}

type KeyboardToRepoSuite struct {
	suite.Suite
}

func TestKeyboardToRepoSuite(t *testing.T) {
	suite.Run(t, new(KeyboardToRepoSuite))
}

func (s *KeyboardToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	orderDate := openapi_types.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	deliveryDate := openapi_types.Date{Time: time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)}
	in := api.KeyboardInput{
		Brand: "Keychron",
		Name:  "Q1",
		Size:  strPtr("75%"),
		Design: &api.KeyboardDesign{
			TopCase:    &api.MaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			BottomCase: &api.MaterialColor{Material: strPtr("Aluminum"), Color: strPtr("Black")},
			Weight:     &api.MaterialColor{Material: strPtr("Brass"), Color: strPtr("Gold")},
			Plates:     &[]string{"FR4", "PC"},
		},
		Pcb: &api.KeyboardPCB{
			Thickness:    floatPtr(1.6),
			Firmware:     strPtr("QMK/VIA"),
			Assembly:     strPtr("Hot-swap"),
			Connectivity: strPtr("Wired"),
		},
		Purchase: &api.Purchase{
			Vendor:       strPtr("Keychron"),
			Price:        floatPtr(199.99),
			OrderDate:    &orderDate,
			DeliveryDate: &deliveryDate,
			OrderStatus:  strPtr("Delivered"),
		},
		Notes:      strPtr("stock lubed"),
		Visibility: api.Private,
	}

	kb := KeyboardToRepo(in)

	s.Empty(kb.UserID, "KeyboardToRepo must not set UserID - that's the handler's job")
	s.Empty(kb.ID, "KeyboardToRepo must not set ID - that's the handler's job")
	s.Equal(in.Brand, kb.Brand)
	s.Equal(in.Name, kb.Name)
	s.Equal(in.Size, kb.Size)
	s.Equal(repository.Visibility(in.Visibility), kb.Visibility)

	s.Equal(in.Design.TopCase.Material, kb.Design.TopCase.Material)
	s.Equal(in.Design.BottomCase.Material, kb.Design.BottomCase.Material)
	s.Equal(in.Design.Weight.Material, kb.Design.Weight.Material)
	s.Equal(*in.Design.Plates, kb.Design.Plates)

	s.Equal(in.Pcb.Thickness, kb.PCB.Thickness)
	s.Equal(in.Pcb.Firmware, kb.PCB.Firmware)
	s.Equal(in.Pcb.Assembly, kb.PCB.Assembly)
	s.Equal(in.Pcb.Connectivity, kb.PCB.Connectivity)

	s.Equal(in.Purchase.Vendor, kb.Purchase.Vendor)
	s.Equal(in.Purchase.Price, kb.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, kb.Purchase.OrderStatus)
	s.Require().NotNil(kb.Purchase.OrderDate)
	s.Equal(in.Purchase.OrderDate.Format(dateLayout), *kb.Purchase.OrderDate)
	s.Require().NotNil(kb.Purchase.DeliveryDate)
	s.Equal(in.Purchase.DeliveryDate.Format(dateLayout), *kb.Purchase.DeliveryDate)
}

func (s *KeyboardToRepoSuite) TestNilSubStructs_ProduceZeroValueStructs() {
	in := api.KeyboardInput{Brand: "Keychron", Name: "Q1", Visibility: api.Private}

	kb := KeyboardToRepo(in)

	s.Equal(repository.KeyboardDesign{}, kb.Design)
	s.Equal(repository.KeyboardPCB{}, kb.PCB)
	s.Equal(repository.KeyboardPurchase{}, kb.Purchase)
}
