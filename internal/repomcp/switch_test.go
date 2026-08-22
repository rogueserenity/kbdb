package repomcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type SwitchToMCPSuite struct {
	suite.Suite
}

func TestSwitchToMCPSuite(t *testing.T) {
	suite.Run(t, new(SwitchToMCPSuite))
}

func (s *SwitchToMCPSuite) TestMapsAllFields() {
	manufacturer := "Gateron"
	pins := 5
	lubed := true
	stem := "POM"
	actuation := 50.0
	springMaterial := "steel"
	vendor := "Divinikey"
	price := 65.0
	orderDate := "2026-01-15"
	deliveryDate := "2026-01-22"
	status := "Delivered"
	quantity := 90
	notes := "smooth"

	sw := repository.Switch{
		ID:           "sw-1",
		Brand:        "Gateron",
		Manufacturer: &manufacturer,
		Name:         "Oil King",
		Type:         "linear",
		Pins:         &pins,
		FactoryLubed: &lubed,
		Material:     repository.SwitchMaterial{Stem: &stem},
		Force:        repository.SwitchForce{Actuation: &actuation},
		Spring:       repository.SwitchSpring{Material: &springMaterial},
		Purchase: repository.SwitchPurchase{
			Vendor: &vendor, Price: &price,
			OrderDate: &orderDate, DeliveryDate: &deliveryDate, OrderStatus: &status,
			Quantity: &quantity,
		},
		Notes:        &notes,
		Visibility:   repository.VisibilityPublic,
	}

	out := SwitchToMCP(sw, true)

	s.Equal("sw-1", out.ID)
	s.Require().NotNil(out.Manufacturer)
	s.Equal("Gateron", *out.Manufacturer)
	s.Require().NotNil(out.Pins)
	s.Equal(5, *out.Pins)
	s.Require().NotNil(out.FactoryLubed)
	s.True(*out.FactoryLubed)
	s.Require().NotNil(out.Material)
	s.Equal("POM", *out.Material.Stem)
	s.Require().NotNil(out.Force)
	s.InDelta(50.0, *out.Force.Actuation, 0.001)
	s.Require().NotNil(out.Spring)
	s.Equal("steel", *out.Spring.Material)
	s.Require().NotNil(out.Purchase)
	s.Equal(90, *out.Purchase.Quantity)
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal("2026-01-15", *out.Purchase.OrderDate)
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal("2026-01-22", *out.Purchase.DeliveryDate)
	s.Require().NotNil(out.Purchase.OrderStatus)
	s.Equal("Delivered", *out.Purchase.OrderStatus)
	s.Require().NotNil(out.Notes)
	s.Equal("smooth", *out.Notes)
	s.Equal("public", out.Visibility)
}

// An all-unset group collapses to nil so it's omitted from the tool result
// entirely, rather than appearing as an object of zero values.
func (s *SwitchToMCPSuite) TestEmptyGroups_CollapseToNil() {
	out := SwitchToMCP(repository.Switch{ID: "sw-1", Visibility: repository.VisibilityPrivate}, true)

	s.Nil(out.Material)
	s.Nil(out.Force)
	s.Nil(out.Spring)
	s.Nil(out.Purchase)
	s.Nil(out.FactoryLubed)
	s.Nil(out.Manufacturer)
	s.Nil(out.Pins)
	s.Nil(out.Notes)
}

// One set field is enough to keep the whole group, so a partially recorded
// group isn't silently dropped.
func (s *SwitchToMCPSuite) TestPartiallySetGroup_IsRetained() {
	bottomOut := 62.0

	out := SwitchToMCP(repository.Switch{
		Force: repository.SwitchForce{BottomOut: &bottomOut},
	}, true)

	s.Require().NotNil(out.Force)
	s.Require().NotNil(out.Force.BottomOut)
	s.InDelta(62.0, *out.Force.BottomOut, 0.001)
	s.Nil(out.Force.Actuation)
}

// A recorded zero (a free purchase, a 0-pin count) must stay distinct from
// an unset field, or MCP would report "not recorded" for a switch REST
// reports as 0.
func (s *SwitchToMCPSuite) TestRecordedZero_SurvivesRoundTrip() {
	pins := 0
	price := 0.0
	quantity := 0

	out := SwitchToMCP(repository.Switch{
		Pins:     &pins,
		Purchase: repository.SwitchPurchase{Price: &price, Quantity: &quantity},
	}, true)

	s.Require().NotNil(out.Pins)
	s.Zero(*out.Pins)
	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.Zero(*out.Purchase.Price)
	s.Require().NotNil(out.Purchase.Quantity)
	s.Zero(*out.Purchase.Quantity)

	raw, err := json.Marshal(out)
	s.Require().NoError(err)
	s.JSONEq(
		`{"id":"","brand":"","name":"","type":"","pins":0,"purchase":{"price":0,"quantity":0},"visibility":""}`,
		string(raw),
	)
}

func (s *SwitchToMCPSuite) TestIsOwnerFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Divinikey"
	status := "Delivered"
	price := 65.0

	out := SwitchToMCP(repository.Switch{
		ID:         "sw-1",
		Purchase:   repository.SwitchPurchase{Vendor: &vendor, OrderStatus: &status, Price: &price},
		Visibility: repository.VisibilityPublic,
	}, false)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(&vendor, out.Purchase.Vendor)
	s.Equal(&status, out.Purchase.OrderStatus)
}

func (s *SwitchToMCPSuite) TestIsOwnerTrue_IncludesPrice() {
	price := 65.0

	out := SwitchToMCP(repository.Switch{
		ID:         "sw-1",
		Purchase:   repository.SwitchPurchase{Price: &price},
		Visibility: repository.VisibilityPublic,
	}, true)

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
}

type SwitchToMCPSummarySuite struct {
	suite.Suite
}

func TestSwitchToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(SwitchToMCPSummarySuite))
}

func (s *SwitchToMCPSummarySuite) TestMapsSummaryFields() {
	notes := "should not appear"

	out := SwitchToMCPSummary(repository.Switch{
		ID:    "sw-1",
		Brand: "Gateron",
		Name:  "Oil King",
		Type:  "linear",
		Notes: &notes,
	})

	s.Equal("sw-1", out.ID)
	s.Equal("Gateron", out.Brand)
	s.Equal("Oil King", out.Name)
	s.Equal("linear", out.Type)
}

type SwitchFromMCPSuite struct {
	suite.Suite
}

func TestSwitchFromMCPSuite(t *testing.T) {
	suite.Run(t, new(SwitchFromMCPSuite))
}

func (s *SwitchFromMCPSuite) TestMapsAllFields() {
	stem := "POM"
	actuation := 50.0
	vendor := "Divinikey"
	price := 0.0
	orderDate := "2026-01-15"
	deliveryDate := "2026-01-22"
	status := "Delivered"

	out := SwitchFromMCP(schema.SwitchInput{
		Brand:    "Gateron",
		Name:     "Oil King",
		Type:     "linear",
		Material: &schema.SwitchMaterial{Stem: &stem},
		Force:    &schema.SwitchForce{Actuation: &actuation},
		Purchase: &schema.SwitchPurchase{
			Vendor: &vendor, Price: &price,
			OrderDate: &orderDate, DeliveryDate: &deliveryDate, OrderStatus: &status,
		},
		Visibility: "public",
	})

	s.Equal("Gateron", out.Brand)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Require().NotNil(out.Material.Stem)
	s.Equal("POM", *out.Material.Stem)
	s.Require().NotNil(out.Force.Actuation)
	s.InDelta(50.0, *out.Force.Actuation, 0.001)
	s.Require().NotNil(out.Purchase.Price)
	s.Zero(*out.Purchase.Price, "a recorded zero price must survive the inbound mapping too")
	s.Require().NotNil(out.Purchase.OrderDate)
	s.Equal("2026-01-15", *out.Purchase.OrderDate)
	s.Require().NotNil(out.Purchase.DeliveryDate)
	s.Equal("2026-01-22", *out.Purchase.DeliveryDate)
	s.Require().NotNil(out.Purchase.OrderStatus)
	s.Equal("Delivered", *out.Purchase.OrderStatus)
}

// ID and UserID are set by the caller and the repository layer
// respectively, never by the tool argument.
func (s *SwitchFromMCPSuite) TestLeavesIdentityUnset() {
	out := SwitchFromMCP(schema.SwitchInput{Brand: "B", Name: "N", Type: "linear", Visibility: "private"})

	s.Empty(out.ID)
	s.Empty(out.UserID)
}

func (s *SwitchFromMCPSuite) TestNilGroups_MapToZeroValues() {
	out := SwitchFromMCP(schema.SwitchInput{Brand: "B", Name: "N", Type: "linear", Visibility: "private"})

	s.Nil(out.Material.Stem)
	s.Nil(out.Force.Actuation)
	s.Nil(out.Spring.Material)
	s.Nil(out.Purchase.Vendor)
	s.Nil(out.Purchase.OrderDate)
	s.Nil(out.Purchase.DeliveryDate)
	s.Nil(out.Purchase.OrderStatus)
}
