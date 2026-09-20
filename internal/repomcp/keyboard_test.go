package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type KeyboardToMCPSuite struct {
	suite.Suite
}

func TestKeyboardToMCPSuite(t *testing.T) {
	suite.Run(t, new(KeyboardToMCPSuite))
}

func (s *KeyboardToMCPSuite) TestMapsAllFields() {
	size := "65%"
	layout := "ANSI"
	material := "Aluminum"
	color := "Silver"
	thickness := 1.6
	firmware := "QMK/VIA"
	vendor := "Divinikey"
	status := "Delivered"
	notes := "daily driver"

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:     "kb-1",
		Brand:  "Mode",
		Name:   "Sixty",
		Size:   &size,
		Layout: &layout,
		Design: repository.KeyboardDesign{
			TopCase: repository.KeyboardMaterialColor{Material: &material, Color: &color},
			Plates:  []string{"Brass", "POM"},
		},
		PCB:        repository.KeyboardPCB{Thickness: &thickness, Firmware: &firmware},
		Purchase:   repository.KeyboardPurchase{Vendor: &vendor, OrderStatus: &status},
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{})

	s.Equal("kb-1", out.ID)
	s.Require().NotNil(out.Size)
	s.Equal("65%", *out.Size)
	s.Require().NotNil(out.Design)
	s.Require().NotNil(out.Design.TopCase)
	s.Equal("Aluminum", *out.Design.TopCase.Material)
	s.Equal([]string{"Brass", "POM"}, out.Design.Plates)
	s.Require().NotNil(out.PCB)
	s.InDelta(1.6, *out.PCB.Thickness, 0.001)
	s.Require().NotNil(out.Purchase)
	s.Equal("Delivered", *out.Purchase.OrderStatus)
	s.Equal("public", out.Visibility)
}

func (s *KeyboardToMCPSuite) TestEmptyGroups_CollapseToNil() {
	out := Keyboard{}.ToMCP(repository.Keyboard{ID: "kb-1", Visibility: repository.VisibilityPrivate}, true, repository.ProfilePreferences{})

	s.Nil(out.Design)
	s.Nil(out.PCB)
	s.Nil(out.Purchase)
	s.Nil(out.Size)
	s.Nil(out.Layout)
	s.Nil(out.Notes)
}

// design collapses only when every part AND plates are empty: plates alone
// is enough to keep the group, since it's a sibling of the three parts
// rather than one of them.
func (s *KeyboardToMCPSuite) TestDesignWithOnlyPlates_IsRetained() {
	out := Keyboard{}.ToMCP(repository.Keyboard{
		Design: repository.KeyboardDesign{Plates: []string{"Brass"}},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Design)
	s.Nil(out.Design.TopCase)
	s.Equal([]string{"Brass"}, out.Design.Plates)
}

func (s *KeyboardToMCPSuite) TestDesignWithOnlyOnePart_IsRetained() {
	material := "PC"

	out := Keyboard{}.ToMCP(repository.Keyboard{
		Design: repository.KeyboardDesign{
			Weight: repository.KeyboardMaterialColor{Material: &material},
		},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Design)
	s.Require().NotNil(out.Design.Weight)
	s.Equal("PC", *out.Design.Weight.Material)
	s.Nil(out.Design.TopCase)
	s.Nil(out.Design.BottomCase)
	s.Empty(out.Design.Plates)
}

// A recorded zero must stay distinct from an unset field, or MCP would
// report "not recorded" for a keyboard REST reports as 0.
func (s *KeyboardToMCPSuite) TestRecordedZero_SurvivesRoundTrip() {
	price := 0.0
	thickness := 0.0

	out := Keyboard{}.ToMCP(repository.Keyboard{
		PCB:      repository.KeyboardPCB{Thickness: &thickness},
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.PCB)
	s.Require().NotNil(out.PCB.Thickness)
	s.Zero(*out.PCB.Thickness)
	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.Zero(*out.Purchase.Price)
}

func (s *KeyboardToMCPSuite) TestNonOwnerShowPriceToOthersFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Divinikey"
	status := "Delivered"
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Vendor: &vendor, OrderStatus: &status, Price: &price},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(&vendor, out.Purchase.Vendor)
	s.Equal(&status, out.Purchase.OrderStatus)
}

func (s *KeyboardToMCPSuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Price: &price},
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
}

func (s *KeyboardToMCPSuite) TestOwner_AlwaysIncludesPriceRegardlessOfShowPriceToMe() {
	price := 199.99

	out := Keyboard{}.ToMCP(repository.Keyboard{
		ID:         "kb-1",
		Purchase:   repository.KeyboardPurchase{Price: &price},
		Visibility: repository.VisibilityPublic,
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
}

type KeyboardToMCPSummarySuite struct {
	suite.Suite
}

func TestKeyboardToMCPSummarySuite(t *testing.T) {
	suite.Run(t, new(KeyboardToMCPSummarySuite))
}

// The summary reaches into purchase for order_status, unlike switches'
// flat summary - a keyboard on order is the case worth surfacing in a list.
func (s *KeyboardToMCPSummarySuite) TestIncludesOrderStatusFromPurchase() {
	size := "TKL"
	status := "Shipped"

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Brand:    "Mode",
		Name:     "Sixty",
		Size:     &size,
		Purchase: repository.KeyboardPurchase{OrderStatus: &status},
	}, true, repository.ProfilePreferences{})

	s.Equal("kb-1", out.ID)
	s.Equal("TKL", *out.Size)
	s.Require().NotNil(out.OrderStatus)
	s.Equal("Shipped", *out.OrderStatus)
}

func (s *KeyboardToMCPSummarySuite) TestNoPurchase_LeavesOrderStatusNil() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{ID: "kb-1"}, true, repository.ProfilePreferences{})

	s.Nil(out.OrderStatus)
}

func (s *KeyboardToMCPSummarySuite) TestOwnerShowPriceToMeTrue_IncludesPrice() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{ShowPriceToMe: true})

	s.Require().NotNil(out.Price)
	s.InDelta(price, *out.Price, 0.0001)
}

func (s *KeyboardToMCPSummarySuite) TestOwnerShowPriceToMeFalse_OmitsPrice() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, true, repository.ProfilePreferences{ShowPriceToMe: false})

	s.Nil(out.Price)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwnerShowPriceToOthersFalse_OmitsPrice() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: false})

	s.Nil(out.Price)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwnerShowPriceToOthersTrue_IncludesPrice() {
	price := 199.99

	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:       "kb-1",
		Purchase: repository.KeyboardPurchase{Price: &price},
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Require().NotNil(out.Price)
	s.InDelta(price, *out.Price, 0.0001)
}

type KeyboardFromMCPSuite struct {
	suite.Suite
}

func TestKeyboardFromMCPSuite(t *testing.T) {
	suite.Run(t, new(KeyboardFromMCPSuite))
}

func (s *KeyboardFromMCPSuite) TestMapsAllFields() {
	size := "60%"
	material := "Aluminum"
	firmware := "QMK/VIA"
	price := 0.0

	out := Keyboard{}.FromMCP(schema.KeyboardInput{
		Brand: "Mode",
		Name:  "Sixty",
		Size:  &size,
		Design: &schema.KeyboardDesign{
			TopCase: &schema.KeyboardMaterialColor{Material: &material},
			Plates:  []string{"Brass"},
		},
		PCB:        &schema.KeyboardPCB{Firmware: &firmware},
		Purchase:   &schema.KeyboardPurchase{Price: &price},
		Visibility: "public",
	})

	s.Equal("Mode", out.Brand)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Equal("60%", *out.Size)
	s.Equal("Aluminum", *out.Design.TopCase.Material)
	s.Equal([]string{"Brass"}, out.Design.Plates)
	s.Equal("QMK/VIA", *out.PCB.Firmware)
	s.Zero(*out.Purchase.Price, "a recorded zero price must survive the inbound mapping too")
}

// ID and UserID are set by the caller and the repository layer
// respectively, never by the tool argument.
func (s *KeyboardFromMCPSuite) TestLeavesIdentityUnset() {
	out := Keyboard{}.FromMCP(schema.KeyboardInput{Brand: "B", Name: "N", Visibility: "private"})

	s.Empty(out.ID)
	s.Empty(out.UserID)
}

func (s *KeyboardFromMCPSuite) TestNilGroups_MapToZeroValues() {
	out := Keyboard{}.FromMCP(schema.KeyboardInput{Brand: "B", Name: "N", Visibility: "private"})

	s.Nil(out.Design.TopCase.Material)
	s.Nil(out.PCB.Firmware)
	s.Nil(out.Purchase.Vendor)
	s.Empty(out.Design.Plates)
}

func (s *KeyboardToMCPSummarySuite) TestOwner_IncludesVisibility() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:         "kb-1",
		Visibility: repository.VisibilityPrivate,
	}, true, repository.ProfilePreferences{})

	s.Require().NotNil(out.Visibility)
	s.Equal("private", *out.Visibility)
}

func (s *KeyboardToMCPSummarySuite) TestNonOwner_OmitsVisibility() {
	out := Keyboard{}.ToMCPSummary(repository.Keyboard{
		ID:         "kb-1",
		Visibility: repository.VisibilityPublic,
	}, false, repository.ProfilePreferences{ShowPriceToOthers: true})

	s.Nil(out.Visibility)
}
