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

func strPtr(s string) *string     { return &s }
func intPtr(i int) *int           { return &i }
func floatPtr(f float64) *float64 { return &f }
func boolPtr(b bool) *bool        { return &b }

func imageKeyPtr(s string) *repository.KeycapKitImageKey {
	k := repository.KeycapKitImageKey(s)
	return &k
}

func fullRepoSwitch() repository.Switch {
	return repository.Switch{
		UserID:       "alice",
		ID:           "sw1",
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: repository.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: repository.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: repository.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
		},
		Purchase: repository.SwitchPurchase{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    strPtr("2026-01-15"),
			DeliveryDate: strPtr("2026-01-22"),
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		},
		Notes:      strPtr("smooth"),
		Visibility: repository.VisibilityPrivate,
	}
}

type SwitchToAPISuite struct {
	suite.Suite
}

func TestSwitchToAPISuite(t *testing.T) {
	suite.Run(t, new(SwitchToAPISuite))
}

func (s *SwitchToAPISuite) TestFullRoundTrip_PreservesEveryField() {
	sw := fullRepoSwitch()
	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Equal(sw.ID, out.Id)
	s.Equal(sw.Brand, out.Brand)
	s.Equal(sw.Manufacturer, out.Manufacturer)
	s.Equal(sw.Name, out.Name)
	s.Equal(sw.Type, out.Type)
	s.Equal(sw.Pins, out.Pins)
	s.Equal(sw.FactoryLubed, out.FactoryLubed)
	s.Equal(sw.Notes, out.Notes)
	s.Equal(api.Visibility(sw.Visibility), out.Visibility)

	if s.NotNil(out.Material) {
		s.Equal(sw.Material.TopHousing, out.Material.TopHousing)
		s.Equal(sw.Material.BottomHousing, out.Material.BottomHousing)
		s.Equal(sw.Material.Stem, out.Material.Stem)
	}
	if s.NotNil(out.Force) {
		s.Equal(sw.Force.Actuation, out.Force.Actuation)
		s.Equal(sw.Force.BottomOut, out.Force.BottomOut)
	}
	if s.NotNil(out.Spring) {
		s.Equal(sw.Spring.Material, out.Spring.Material)
		s.Equal(sw.Spring.PreTravel, out.Spring.PreTravel)
		s.Equal(sw.Spring.TotalTravel, out.Spring.TotalTravel)
	}
	if s.NotNil(out.Purchase) {
		s.Equal(sw.Purchase.Vendor, out.Purchase.Vendor)
		s.Equal(sw.Purchase.Price, out.Purchase.Price)
		s.Equal(sw.Purchase.OrderStatus, out.Purchase.OrderStatus)
		s.Equal(sw.Purchase.Quantity, out.Purchase.Quantity)
		s.Require().NotNil(out.Purchase.OrderDate)
		s.Equal(*sw.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
		s.Require().NotNil(out.Purchase.DeliveryDate)
		s.Equal(*sw.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
	}
}

func (s *SwitchToAPISuite) TestAllOptionalFieldsNil_SubStructsOmitted() {
	sw := repository.Switch{ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate}

	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Nil(out.Manufacturer)
	s.Nil(out.Pins)
	s.Nil(out.FactoryLubed)
	s.Nil(out.Notes)
	s.Nil(out.Material, "an all-nil SwitchMaterial must map to a nil pointer, not an empty object")
	s.Nil(out.Force, "an all-nil SwitchForce must map to a nil pointer, not an empty object")
	s.Nil(out.Spring, "an all-nil SwitchSpring must map to a nil pointer, not an empty object")
	s.Nil(out.Purchase, "an all-nil SwitchPurchase must map to a nil pointer, not an empty object")
}

func (s *SwitchToAPISuite) TestOneFieldSetInSubStruct_SubStructPresent() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Material: repository.SwitchMaterial{Stem: strPtr("POM")},
	}

	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)
	s.Require().NoError(err)

	if s.NotNil(out.Material) {
		s.Nil(out.Material.TopHousing)
		s.Nil(out.Material.BottomHousing)
		s.Equal(strPtr("POM"), out.Material.Stem)
	}
}

func (s *SwitchToAPISuite) TestMalformedStoredDate_ReturnsError() {
	sw := repository.Switch{
		ID: "sw1", Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: repository.VisibilityPrivate,
		Purchase: repository.SwitchPurchase{OrderDate: strPtr("not-a-date")},
	}

	_, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)

	s.Require().Error(err)
}

func (s *SwitchToAPISuite) TestIsOwnerFalse_OmitsPriceKeepsRestOfPurchase() {
	sw := fullRepoSwitch()

	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), false)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(sw.Purchase.Vendor, out.Purchase.Vendor)
	s.Equal(sw.Purchase.OrderStatus, out.Purchase.OrderStatus)
	s.Equal(sw.Purchase.Quantity, out.Purchase.Quantity)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal(*sw.Purchase.OrderDate, out.Purchase.OrderDate.Format(dateLayout))
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal(*sw.Purchase.DeliveryDate, out.Purchase.DeliveryDate.Format(dateLayout))
}

func (s *SwitchToAPISuite) TestIsOwnerTrue_IncludesPrice() {
	sw := fullRepoSwitch()

	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Purchase)
	s.Equal(sw.Purchase.Price, out.Purchase.Price)
}

func (s *SwitchToAPISuite) TestSwitchToAPISummary_MapsOnlySummaryFields() {
	sw := fullRepoSwitch()
	images := mocks.NewMockSwitchImageStore(s.T())

	summary, err := SwitchToAPISummary(s.T().Context(), sw, images)
	s.Require().NoError(err)

	s.Equal(&sw.ID, summary.Id)
	s.Equal(&sw.Brand, summary.Brand)
	s.Equal(&sw.Name, summary.Name)
	s.Equal(&sw.Type, summary.Type)
	s.Equal(sw.Purchase.OrderStatus, summary.OrderStatus)
	s.Nil(summary.Image, "no image on the switch must map to a nil Image")
}

func (s *SwitchToAPISuite) TestSwitchToAPISummary_ImagePresent_ReturnsPresignedURL() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *sw.ImagePath).Return("https://example.com/img", nil)

	summary, err := SwitchToAPISummary(s.T().Context(), sw, images)
	s.Require().NoError(err)

	s.Require().NotNil(summary.Image)
	s.Equal("https://example.com/img", summary.Image.Url)
}

func (s *SwitchToAPISuite) TestSwitchToAPISummary_PresignError_Propagates() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *sw.ImagePath).Return("", errors.New("s3: access denied"))

	_, err := SwitchToAPISummary(s.T().Context(), sw, images)

	s.Require().Error(err)
}

func (s *SwitchToAPISuite) TestImagePresent_ReturnsPresignedURL() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *sw.ImagePath).Return("https://example.com/img", nil)

	out, err := SwitchToAPI(s.T().Context(), sw, images, true)
	s.Require().NoError(err)

	s.Require().NotNil(out.Image)
	s.Equal("https://example.com/img", out.Image.Url)
}

func (s *SwitchToAPISuite) TestNoImage_ImageFieldNil() {
	sw := fullRepoSwitch()

	out, err := SwitchToAPI(s.T().Context(), sw, mocks.NewMockSwitchImageStore(s.T()), true)
	s.Require().NoError(err)

	s.Nil(out.Image)
}

func (s *SwitchToAPISuite) TestImagePresignError_Propagates() {
	sw := fullRepoSwitch()
	switchImageKey := repository.SwitchImageKey("switches/alice/sw1/image")
	sw.ImagePath = &switchImageKey
	images := mocks.NewMockSwitchImageStore(s.T())
	images.EXPECT().PresignGet(mock.Anything, *sw.ImagePath).Return("", errors.New("s3: access denied"))

	_, err := SwitchToAPI(s.T().Context(), sw, images, true)

	s.Require().Error(err)
}

type SwitchToRepoSuite struct {
	suite.Suite
}

func TestSwitchToRepoSuite(t *testing.T) {
	suite.Run(t, new(SwitchToRepoSuite))
}

func (s *SwitchToRepoSuite) TestFullRoundTrip_PreservesEveryField() {
	orderDate := openapi_types.Date{Time: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)}
	deliveryDate := openapi_types.Date{Time: time.Date(2026, 1, 22, 0, 0, 0, 0, time.UTC)}
	in := api.SwitchInput{
		Brand:        "Gateron",
		Manufacturer: strPtr("Gateron Inc"),
		Name:         "Yellow",
		Type:         "Linear",
		Pins:         intPtr(5),
		FactoryLubed: boolPtr(true),
		Material: &api.SwitchMaterial{
			TopHousing:    strPtr("PC"),
			BottomHousing: strPtr("Nylon"),
			Stem:          strPtr("POM"),
		},
		Force: &api.SwitchForce{
			Actuation: floatPtr(50),
			BottomOut: floatPtr(60),
		},
		Spring: &api.SwitchSpring{
			Material:    strPtr("Steel"),
			PreTravel:   floatPtr(2),
			TotalTravel: floatPtr(4),
		},
		Purchase: &api.SwitchPurchase{
			Vendor:       strPtr("CannonKeys"),
			Price:        floatPtr(0.35),
			OrderDate:    &orderDate,
			DeliveryDate: &deliveryDate,
			OrderStatus:  strPtr("Delivered"),
			Quantity:     intPtr(90),
		},
		Notes:      strPtr("smooth"),
		Visibility: api.Private,
	}

	sw := SwitchToRepo(in)

	s.Empty(sw.UserID, "SwitchToRepo must not set UserID - that's the handler's job")
	s.Empty(sw.ID, "SwitchToRepo must not set ID - that's the handler's job")
	s.Equal(in.Brand, sw.Brand)
	s.Equal(in.Manufacturer, sw.Manufacturer)
	s.Equal(in.Name, sw.Name)
	s.Equal(in.Type, sw.Type)
	s.Equal(in.Pins, sw.Pins)
	s.Equal(in.FactoryLubed, sw.FactoryLubed)
	s.Equal(in.Notes, sw.Notes)
	s.Equal(repository.Visibility(in.Visibility), sw.Visibility)

	s.Equal(in.Material.TopHousing, sw.Material.TopHousing)
	s.Equal(in.Material.BottomHousing, sw.Material.BottomHousing)
	s.Equal(in.Material.Stem, sw.Material.Stem)
	s.Equal(in.Force.Actuation, sw.Force.Actuation)
	s.Equal(in.Force.BottomOut, sw.Force.BottomOut)
	s.Equal(in.Spring.Material, sw.Spring.Material)
	s.Equal(in.Spring.PreTravel, sw.Spring.PreTravel)
	s.Equal(in.Spring.TotalTravel, sw.Spring.TotalTravel)
	s.Equal(in.Purchase.Vendor, sw.Purchase.Vendor)
	s.Equal(in.Purchase.Price, sw.Purchase.Price)
	s.Equal(in.Purchase.OrderStatus, sw.Purchase.OrderStatus)
	s.Equal(in.Purchase.Quantity, sw.Purchase.Quantity)
	s.Require().NotNil(sw.Purchase.OrderDate)
	s.Equal(in.Purchase.OrderDate.Format(dateLayout), *sw.Purchase.OrderDate)
	s.Require().NotNil(sw.Purchase.DeliveryDate)
	s.Equal(in.Purchase.DeliveryDate.Format(dateLayout), *sw.Purchase.DeliveryDate)
}

func (s *SwitchToRepoSuite) TestNilSubStructs_ProduceZeroValueStructs() {
	in := api.SwitchInput{Brand: "Gateron", Name: "Yellow", Type: "Linear", Visibility: api.Private}

	sw := SwitchToRepo(in)

	s.Equal(repository.SwitchMaterial{}, sw.Material)
	s.Equal(repository.SwitchForce{}, sw.Force)
	s.Equal(repository.SwitchSpring{}, sw.Spring)
	s.Equal(repository.SwitchPurchase{}, sw.Purchase)
}
