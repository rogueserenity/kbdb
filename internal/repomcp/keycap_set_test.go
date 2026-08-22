package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type KeycapSetToMCPSuite struct {
	suite.Suite
}

func TestKeycapSetToMCPSuite(t *testing.T) {
	suite.Run(t, new(KeycapSetToMCPSuite))
}

func (s *KeycapSetToMCPSuite) TestMapsAllFields() {
	profile := "Cherry"
	material := "ABS"
	notes := "grail set"
	imagePath := repository.KeycapKitImageKey("keycap-sets/u-1/ks-1/kits/kit-1/image")
	vendor := "Amazon"

	out := KeycapSetToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Brand:      "GMK",
		Name:       "Olivia",
		Profile:    &profile,
		Material:   &material,
		Notes:      &notes,
		Visibility: repository.VisibilityPublic,
		Kits: []repository.KeycapKit{
			{
				KitID:     "kit-1",
				Name:      "Base",
				ImagePath: &imagePath,
				Purchase:  repository.KeycapKitPurchase{Vendor: &vendor},
			},
			{KitID: "kit-2", Name: "Novelties"},
		},
	}, true)

	s.Equal("ks-1", out.ID)
	s.Equal("GMK", out.Brand)
	s.Require().NotNil(out.Profile)
	s.Equal("Cherry", *out.Profile)
	s.Equal("public", out.Visibility)
	s.Require().Len(out.Kits, 2)

	s.Equal("kit-1", out.Kits[0].KitID)
	s.True(out.Kits[0].HasImage)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Equal("Amazon", *out.Kits[0].Purchase.Vendor)

	s.Equal("kit-2", out.Kits[1].KitID)
	s.False(out.Kits[1].HasImage)
	s.Nil(out.Kits[1].Purchase)
}

func (s *KeycapSetToMCPSuite) TestNilKits_MapsToNilSlice() {
	out := KeycapSetToMCP(repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate}, true)

	s.Nil(out.Kits)
}

func (s *KeycapSetToMCPSuite) TestIsOwnerFalse_OmitsKitPriceKeepsRestOfPurchase() {
	vendor := "Amazon"
	price := 120.0

	out := KeycapSetToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
		Kits: []repository.KeycapKit{
			{KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Vendor: &vendor, Price: &price}},
		},
	}, false)

	s.Require().Len(out.Kits, 1)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Nil(out.Kits[0].Purchase.Price)
	s.Equal(&vendor, out.Kits[0].Purchase.Vendor)
}

func (s *KeycapSetToMCPSuite) TestIsOwnerTrue_IncludesKitPrice() {
	price := 120.0

	out := KeycapSetToMCP(repository.KeycapSet{
		ID:         "ks-1",
		Visibility: repository.VisibilityPublic,
		Kits: []repository.KeycapKit{
			{KitID: "kit-1", Name: "Base", Purchase: repository.KeycapKitPurchase{Price: &price}},
		},
	}, true)

	s.Require().Len(out.Kits, 1)
	s.Require().NotNil(out.Kits[0].Purchase)
	s.Require().NotNil(out.Kits[0].Purchase.Price)
	s.InDelta(price, *out.Kits[0].Purchase.Price, 0.0001)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetToMCPSummary() {
	profile := "OEM"
	out := KeycapSetToMCPSummary(repository.KeycapSet{
		ID:      "ks-1",
		Brand:   "GMK",
		Name:    "Olivia",
		Profile: &profile,
	})

	s.Equal("ks-1", out.ID)
	s.Equal("GMK", out.Brand)
	s.Require().NotNil(out.Profile)
	s.Equal("OEM", *out.Profile)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_NoPurchaseFields_OmitsPurchase() {
	out := KeycapKitToMCP(repository.KeycapKit{KitID: "kit-1", Name: "Base"}, true)

	s.Equal("kit-1", out.KitID)
	s.False(out.HasImage)
	s.Nil(out.Purchase)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_IsOwnerFalse_OmitsPriceKeepsRestOfPurchase() {
	vendor := "Amazon"
	price := 120.0

	out := KeycapKitToMCP(repository.KeycapKit{
		KitID:    "kit-1",
		Name:     "Base",
		Purchase: repository.KeycapKitPurchase{Vendor: &vendor, Price: &price},
	}, false)

	s.Require().NotNil(out.Purchase)
	s.Nil(out.Purchase.Price)
	s.Equal(&vendor, out.Purchase.Vendor)
}

func (s *KeycapSetToMCPSuite) TestKeycapKitToMCP_IsOwnerTrue_IncludesPrice() {
	price := 120.0

	out := KeycapKitToMCP(repository.KeycapKit{
		KitID:    "kit-1",
		Name:     "Base",
		Purchase: repository.KeycapKitPurchase{Price: &price},
	}, true)

	s.Require().NotNil(out.Purchase)
	s.Require().NotNil(out.Purchase.Price)
	s.InDelta(price, *out.Purchase.Price, 0.0001)
}

func (s *KeycapSetToMCPSuite) TestKeycapSetFromMCP_MapsAllFields() {
	profile := "Cherry"
	material := "ABS"
	notes := "grail set"

	out := KeycapSetFromMCP(schema.KeycapSetInput{
		Brand:      "GMK",
		Name:       "Olivia",
		Profile:    &profile,
		Material:   &material,
		Notes:      &notes,
		Visibility: "public",
	})

	s.Equal("GMK", out.Brand)
	s.Equal("Olivia", out.Name)
	s.Require().NotNil(out.Profile)
	s.Equal("Cherry", *out.Profile)
	s.Equal(repository.VisibilityPublic, out.Visibility)
	s.Empty(out.ID, "ID is the caller's responsibility, not this mapping's")
	s.Nil(out.Kits, "a set write never carries kits")
}

func (s *KeycapSetToMCPSuite) TestKeycapKitFromMCP_MapsAllFields() {
	vendor := "Amazon"
	price := 120.0
	orderDate := "2026-01-15"

	out := KeycapKitFromMCP(schema.KeycapKitInput{
		Name: "Base",
		Purchase: &schema.KeycapKitPurchase{
			Vendor:    &vendor,
			Price:     &price,
			OrderDate: &orderDate,
		},
	})

	s.Equal("Base", out.Name)
	s.Require().NotNil(out.Purchase.Vendor)
	s.Equal("Amazon", *out.Purchase.Vendor)
	s.Require().NotNil(out.Purchase.Price)
	s.InEpsilon(120.0, *out.Purchase.Price, 0.001)
	s.Empty(out.KitID, "KitID is the caller's responsibility, not this mapping's")
	s.Nil(out.ImagePath, "a kit write never carries its image path")
}

func (s *KeycapSetToMCPSuite) TestKeycapKitFromMCP_NoPurchase_MapsToZeroValue() {
	out := KeycapKitFromMCP(schema.KeycapKitInput{Name: "Base"})

	s.Equal(repository.KeycapKitPurchase{}, out.Purchase)
}
