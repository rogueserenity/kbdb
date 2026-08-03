package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ValidateKeycapSetSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateKeycapSetSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeycapSetSuite))
}

func (s *ValidateKeycapSetSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateKeycapSetSuite) TestAllFieldsUnset_SkipsValidation() {
	ks := repository.KeycapSet{}

	errs, err := lookup.ValidateKeycapSet(s.T().Context(), s.mockRepo, ks)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateKeycapSetSuite) TestInvalidProfile_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryKeycapProfile).
		Return(&repository.Lookup{Category: repository.CategoryKeycapProfile, Values: []any{"Cherry"}}, nil)

	profile := "OEM"
	ks := repository.KeycapSet{Profile: &profile}

	errs, err := lookup.ValidateKeycapSet(s.T().Context(), s.mockRepo, ks)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "profile", Value: "OEM", Category: repository.CategoryKeycapProfile},
	}, errs)
}
