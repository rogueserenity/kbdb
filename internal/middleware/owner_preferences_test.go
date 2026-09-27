package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/ownerprefs"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type OwnerPreferencesSuite struct {
	suite.Suite
}

func TestOwnerPreferencesSuite(t *testing.T) {
	suite.Run(t, new(OwnerPreferencesSuite))
}

func (s *OwnerPreferencesSuite) TestLoadsPathOwnersPreferences() {
	reader := mocks.NewMockPreferencesReader(s.T())
	want := repository.ProfilePreferences{Currency: "EUR"}
	reader.EXPECT().GetPreferences(mock.Anything, "alice").Return(want, nil)

	var got repository.ProfilePreferences
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		got, err = ownerprefs.Get(r.Context())
		s.NoError(err)
	})

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodGet, "/v1/users/alice/switches", nil)
	req.SetPathValue("userId", "alice")
	OwnerPreferences(reader)(next).ServeHTTP(httptest.NewRecorder(), req)

	s.Equal(want, got)
}

func (s *OwnerPreferencesSuite) TestNextNeverReadsPreferences_DoesNotFetch() {
	reader := mocks.NewMockPreferencesReader(s.T())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequestWithContext(s.T().Context(), http.MethodDelete, "/v1/users/alice/switches/sw1", nil)
	req.SetPathValue("userId", "alice")
	rec := httptest.NewRecorder()
	OwnerPreferences(reader)(next).ServeHTTP(rec, req)

	s.Equal(http.StatusNoContent, rec.Code)
}
