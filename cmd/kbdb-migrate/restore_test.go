package main

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
)

type RestoreSuite struct {
	suite.Suite
}

func TestRestoreSuite(t *testing.T) {
	suite.Run(t, new(RestoreSuite))
}

// fullMap is an id map with one keyboard, one switch, and one keycap set (with
// one kit) already restored.
func (s *RestoreSuite) fullMap() *idMap {
	return &idMap{
		Keyboards: map[string]*mappedEntity{"kb-old": {NewID: "kb-new"}},
		Switches:  map[string]*mappedEntity{"sw-old": {NewID: "sw-new"}},
		KeycapSets: map[string]*mappedKeycaps{
			"set-old": {NewID: "set-new", Kits: map[string]string{"kit-old": "kit-new"}},
		},
		Builds: map[string]*mappedEntity{},
	}
}

func (s *RestoreSuite) TestBuildInputFromResolved_RemapsEveryReference() {
	brass := "brass"
	full := api.Build{
		Id:         "b-old",
		Keyboard:   &api.BuildKeyboardRef{Id: "kb-old", Brand: "B", Name: "N"},
		Plate:      &brass,
		Visibility: "public",
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 70, Switch: &api.BuildSwitchRef{Id: "sw-old", Name: "S", Type: "linear"}},
		},
		KeycapKits: &[]api.BuildKeycapKitEntryResolved{
			{KitId: "kit-old", KeycapSet: &api.BuildKeycapSetRef{Id: "set-old", Brand: "GMK", Name: "X"}},
		},
	}

	got, err := buildInputFromResolved(full, s.fullMap())
	s.Require().NoError(err)
	s.Equal("kb-new", got.Keyboard)
	s.Equal("brass", *got.Plate)
	s.Equal("public", string(got.Visibility))

	s.Require().NotNil(got.Switches)
	s.Require().Len(*got.Switches, 1)
	s.Equal("sw-new", (*got.Switches)[0].Switch)
	s.Equal(70, (*got.Switches)[0].Count)

	s.Require().NotNil(got.KeycapKits)
	s.Require().Len(*got.KeycapKits, 1)
	s.Equal("set-new", (*got.KeycapKits)[0].KeycapSet)
	s.Equal("kit-new", (*got.KeycapKits)[0].Kit)
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedKeyboardIsError() {
	full := api.Build{Keyboard: &api.BuildKeyboardRef{Id: "kb-unknown"}, Visibility: "public"}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kb-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedSwitchIsError() {
	full := api.Build{
		Keyboard:   &api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: "public",
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 1, Switch: &api.BuildSwitchRef{Id: "sw-unknown"}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "sw-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedKitIsError() {
	full := api.Build{
		Keyboard:   &api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: "public",
		KeycapKits: &[]api.BuildKeycapKitEntryResolved{
			{KitId: "kit-unknown", KeycapSet: &api.BuildKeycapSetRef{Id: "set-old"}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kit-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_MissingKeyboardIsError() {
	_, err := buildInputFromResolved(api.Build{Visibility: "public"}, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "no keyboard")
}
