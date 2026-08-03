package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
	"github.com/rogueserenity/kbdb/internal/repository/mocks"
)

type ValidateSwitchSuite struct {
	suite.Suite

	mockRepo *mocks.MockLookupRepository
}

func TestValidateSwitchSuite(t *testing.T) {
	suite.Run(t, new(ValidateSwitchSuite))
}

func (s *ValidateSwitchSuite) SetupTest() {
	s.mockRepo = mocks.NewMockLookupRepository(s.T())
}

func (s *ValidateSwitchSuite) TestOptionalFieldsUnset_SkipsValidation() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategorySwitchType).
		Return(&repository.Lookup{Category: repository.CategorySwitchType, Values: []any{"Linear"}}, nil)

	sw := repository.Switch{Type: "Linear"}

	errs, err := lookup.ValidateSwitch(s.T().Context(), s.mockRepo, sw)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestValidFields_ReturnsNoErrors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategorySwitchType).
		Return(&repository.Lookup{Category: repository.CategorySwitchType, Values: []any{"Linear"}}, nil)
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategoryVendor).
		Return(&repository.Lookup{Category: repository.CategoryVendor, Values: []any{"NovelKeys"}}, nil)

	vendor := "NovelKeys"
	sw := repository.Switch{
		Type:     "Linear",
		Purchase: repository.SwitchPurchase{Vendor: &vendor},
	}

	errs, err := lookup.ValidateSwitch(s.T().Context(), s.mockRepo, sw)
	s.Require().NoError(err)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestCategoryValuesWrongShape_Errors() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategorySwitchType).
		Return(&repository.Lookup{Category: repository.CategorySwitchType, Values: []any{123}}, nil)

	sw := repository.Switch{Type: "Linear"}

	_, err := lookup.ValidateSwitch(s.T().Context(), s.mockRepo, sw)
	s.Require().Error(err)
}

func (s *ValidateSwitchSuite) TestInvalidType_ReturnsFieldError() {
	s.mockRepo.EXPECT().
		GetCategory(mock.Anything, repository.CategorySwitchType).
		Return(&repository.Lookup{Category: repository.CategorySwitchType, Values: []any{"Linear"}}, nil)

	sw := repository.Switch{Type: "Tactile"}

	errs, err := lookup.ValidateSwitch(s.T().Context(), s.mockRepo, sw)
	s.Require().NoError(err)
	s.Equal([]lookup.FieldError{
		{Field: "type", Value: "Tactile", Category: repository.CategorySwitchType},
	}, errs)
}
