package repomcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

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
		Purchase:     repository.SwitchPurchase{Vendor: &vendor, Price: &price, Quantity: &quantity},
		Notes:        &notes,
		Visibility:   repository.VisibilityPublic,
	}

	out := SwitchToMCP(sw)

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
	s.Require().NotNil(out.Notes)
	s.Equal("smooth", *out.Notes)
	s.Equal("public", out.Visibility)
}

// An all-unset group collapses to nil so it's omitted from the tool result
// entirely, rather than appearing as an object of zero values.
func (s *SwitchToMCPSuite) TestEmptyGroups_CollapseToNil() {
	out := SwitchToMCP(repository.Switch{ID: "sw-1", Visibility: repository.VisibilityPrivate})

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
	})

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
	})

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
