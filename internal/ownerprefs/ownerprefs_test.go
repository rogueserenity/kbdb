package ownerprefs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type OwnerPrefsSuite struct {
	suite.Suite
}

func TestOwnerPrefsSuite(t *testing.T) {
	suite.Run(t, new(OwnerPrefsSuite))
}

func (s *OwnerPrefsSuite) TestGet_NoLoader_ReturnsErrNoLoader() {
	_, err := Get(s.T().Context())

	s.Require().ErrorIs(err, ErrNoLoader)
}

func (s *OwnerPrefsSuite) TestGet_FetchesOwnersPreferences() {
	reader := mocks.NewMockPreferencesReader(s.T())
	want := repository.ProfilePreferences{Currency: "EUR", ShowPriceToOthers: true}
	reader.EXPECT().GetPreferences(mock.Anything, "alice").Return(want, nil).Once()

	got, err := Get(WithLoader(s.T().Context(), reader, "alice"))

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *OwnerPrefsSuite) TestGet_CalledTwice_FetchesOnce() {
	reader := mocks.NewMockPreferencesReader(s.T())
	want := repository.ProfilePreferences{Currency: "EUR"}
	reader.EXPECT().GetPreferences(mock.Anything, "alice").Return(want, nil).Once()
	ctx := WithLoader(s.T().Context(), reader, "alice")

	_, err := Get(ctx)
	s.Require().NoError(err)
	got, err := Get(ctx)

	s.Require().NoError(err)
	s.Equal(want, got)
}

func (s *OwnerPrefsSuite) TestGet_FetchError_ReturnedOnEveryCall() {
	reader := mocks.NewMockPreferencesReader(s.T())
	reader.EXPECT().GetPreferences(mock.Anything, "alice").Return(repository.ProfilePreferences{}, errors.New("boom")).Once()
	ctx := WithLoader(s.T().Context(), reader, "alice")

	_, err := Get(ctx)
	s.Require().EqualError(err, "boom")
	_, err = Get(ctx)

	s.Require().EqualError(err, "boom")
}

func (s *OwnerPrefsSuite) TestWithLoader_NeverRead_DoesNotFetch() {
	reader := mocks.NewMockPreferencesReader(s.T())

	_ = WithLoader(s.T().Context(), reader, "alice")
}
