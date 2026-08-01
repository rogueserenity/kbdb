package repoapi

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
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
	out := KeycapSetToAPI(ks)

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

	out := KeycapSetToAPI(ks)

	s.Nil(out.Profile)
	s.Nil(out.Material)
	s.Nil(out.Notes)
}

func (s *KeycapSetToAPISuite) TestKeycapSetToAPISummary_MapsOnlySummaryFields() {
	ks := fullRepoKeycapSet()

	summary := KeycapSetToAPISummary(ks)

	s.Equal(&ks.ID, summary.Id)
	s.Equal(&ks.Brand, summary.Brand)
	s.Equal(&ks.Name, summary.Name)
	s.Equal(ks.Profile, summary.Profile)
}
