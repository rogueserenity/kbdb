package repomcp

import (
	"testing"

	"github.com/stretchr/testify/suite"

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
	})

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
	out := KeycapSetToMCP(repository.KeycapSet{ID: "ks-1", Visibility: repository.VisibilityPrivate})

	s.Nil(out.Kits)
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
	out := KeycapKitToMCP(repository.KeycapKit{KitID: "kit-1", Name: "Base"})

	s.Equal("kit-1", out.KitID)
	s.False(out.HasImage)
	s.Nil(out.Purchase)
}
