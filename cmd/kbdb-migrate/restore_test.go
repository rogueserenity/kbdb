package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		Keyboards: map[string]*mappedEntity{"kb-old": {
			NewID:  "kb-new",
			Plates: map[string]string{"plate-old": "plate-new"},
			PCBs:   map[string]string{"pcb-old": "pcb-new"},
		}},
		Switches:  map[string]*mappedEntity{"sw-old": {NewID: "sw-new"}},
		KeycapSets: map[string]*mappedKeycaps{
			"set-old": {NewID: "set-new", Kits: map[string]string{"kit-old": "kit-new"}},
		},
		Builds: map[string]*mappedEntity{},
	}
}

func (s *RestoreSuite) TestBuildInputFromResolved_RemapsEveryReference() {
	full := api.Build{
		Id:         "b-old",
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old", Brand: "B", Name: "N"},
		Plate:      &api.BuildPlateRef{Id: "plate-old", Material: "Brass"},
		Pcb:        &api.BuildPCBRef{Id: "pcb-old"},
		Visibility: new(api.Visibility("public")),
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 70, Switch: api.BuildSwitchRef{Id: "sw-old", Name: "S", Type: "linear"}},
		},
		KeycapSets: &[]api.BuildKeycapSetRef{
			{Id: "set-old", Brand: "GMK", Name: "X", Kits: []api.BuildKeycapKitRef{{KitId: "kit-old", Name: "Base"}}},
		},
	}

	got, err := buildInputFromResolved(full, s.fullMap())
	s.Require().NoError(err)
	s.Equal("kb-new", got.Keyboard)
	s.Equal(new("plate-new"), got.Plate)
	s.Equal(new("pcb-new"), got.Pcb)
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
	full := api.Build{Keyboard: api.BuildKeyboardRef{Id: "kb-unknown"}, Visibility: new(api.Visibility("public"))}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kb-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedPlateIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		Plate:      &api.BuildPlateRef{Id: "plate-unknown", Material: "AL"},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "plate-unknown")
}

func (s *RestoreSuite) TestMapPartIDs_PairsByPosition() {
	dumped := &[]api.KeyboardPlate{{Id: "AL", Material: "AL"}, {Id: "AL#2", Material: "AL"}}
	created := &[]api.KeyboardPlate{{Id: "n1", Material: "AL"}, {Id: "n2", Material: "AL"}}

	got, err := mapPartIDs(dumped, created, func(p api.KeyboardPlate) string { return p.Id })

	s.Require().NoError(err)
	s.Equal(map[string]string{"AL": "n1", "AL#2": "n2"}, got)
}

func (s *RestoreSuite) TestMapPartIDs_CountMismatchIsError() {
	dumped := &[]api.KeyboardPCB{{Id: "pcb"}}

	_, err := mapPartIDs(dumped, nil, func(p api.KeyboardPCB) string { return p.Id })

	s.Require().Error(err)
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedSwitchIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		Switches: &[]api.BuildSwitchEntryResolved{
			{Count: 1, Switch: api.BuildSwitchRef{Id: "sw-unknown"}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "sw-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_UnmappedKitIsError() {
	full := api.Build{
		Keyboard:   api.BuildKeyboardRef{Id: "kb-old"},
		Visibility: new(api.Visibility("public")),
		KeycapSets: &[]api.BuildKeycapSetRef{
			{Id: "set-old", Kits: []api.BuildKeycapKitRef{{KitId: "kit-unknown"}}},
		},
	}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "kit-unknown")
}

func (s *RestoreSuite) TestBuildInputFromResolved_MissingVisibilityIsError() {
	full := api.Build{Keyboard: api.BuildKeyboardRef{Id: "kb-old"}}
	_, err := buildInputFromResolved(full, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "no visibility")
}

func (s *RestoreSuite) TestBuildInputFromResolved_MissingKeyboardIsError() {
	_, err := buildInputFromResolved(api.Build{Visibility: new(api.Visibility("public"))}, s.fullMap())
	s.Require().Error(err)
	s.ErrorContains(err, "no keyboard")
}

func (s *RestoreSuite) TestRestoreKeyboards_PartCountMismatchStillRecordsTheKeyboard() {
	dumpDir := s.T().TempDir()
	itemDir := filepath.Join(dumpDir, "keyboards", "kb-old")
	s.Require().NoError(os.MkdirAll(itemDir, 0o750))
	s.Require().NoError(os.WriteFile(filepath.Join(itemDir, "item.json"), []byte(
		`{"id":"kb-old","brand":"Acme","name":"One","visibility":"private","plates":[{"id":"plate-old","material":"FR4"}]}`,
	), 0o600))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"kb-new","brand":"Acme","name":"One","visibility":"private"}`))
	}))
	defer srv.Close()

	client := &apiClient{baseURL: srv.URL, subject: "u-1", http: srv.Client()}
	m := newIDMap(s.T().TempDir(), "u-1")

	err := restoreKeyboards(context.Background(), client, dumpDir, m)

	s.Require().ErrorContains(err, "plates")
	s.Require().Contains(m.Keyboards, "kb-old")
	s.Equal("kb-new", m.Keyboards["kb-old"].NewID)
	saved, err := os.ReadFile(m.path)
	s.Require().NoError(err)
	s.Contains(string(saved), `"kb-new"`)
}
