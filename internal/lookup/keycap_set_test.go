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

type ValidateKeycapKitSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateKeycapKitSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeycapKitSuite))
}

func (s *ValidateKeycapKitSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateKeycapKitSuite) TestAllFieldsUnset_SkipsValidation() {
	k := repository.KeycapKit{}

	errs, err := lookup.ValidateKeycapKit(s.T().Context(), s.mockRepo, k)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateKeycapKitSuite) TestValidFields_ReturnsNoErrors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryVendor).
		Return(&repository.Lookup{Category: repository.CategoryVendor, Values: []any{"NovelKeys"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryOrderStatus).
		Return(&repository.Lookup{Category: repository.CategoryOrderStatus, Values: []any{"Delivered"}}, nil)

	vendor := "NovelKeys"
	status := "Delivered"
	k := repository.KeycapKit{Purchase: repository.KeycapKitPurchase{Vendor: &vendor, OrderStatus: &status}}

	errs, err := lookup.ValidateKeycapKit(s.T().Context(), s.mockRepo, k)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateKeycapKitSuite) TestInvalidVendor_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryVendor).
		Return(&repository.Lookup{Category: repository.CategoryVendor, Values: []any{"NovelKeys"}}, nil)

	vendor := "NotARealVendor"
	k := repository.KeycapKit{Purchase: repository.KeycapKitPurchase{Vendor: &vendor}}

	errs, err := lookup.ValidateKeycapKit(s.T().Context(), s.mockRepo, k)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "purchase.vendor", Value: "NotARealVendor", Category: repository.CategoryVendor},
	}, errs)
}

func (s *ValidateKeycapKitSuite) TestInvalidOrderStatus_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryOrderStatus).
		Return(&repository.Lookup{Category: repository.CategoryOrderStatus, Values: []any{"Delivered"}}, nil)

	status := "Bogus"
	k := repository.KeycapKit{Purchase: repository.KeycapKitPurchase{OrderStatus: &status}}

	errs, err := lookup.ValidateKeycapKit(s.T().Context(), s.mockRepo, k)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "purchase.order_status", Value: "Bogus", Category: repository.CategoryOrderStatus},
	}, errs)
}
