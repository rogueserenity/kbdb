package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type ValidateSwitchSuite struct {
	suite.Suite
}

func TestValidateSwitchSuite(t *testing.T) {
	suite.Run(t, new(ValidateSwitchSuite))
}

func (s *ValidateSwitchSuite) TestOptionalFieldsUnset_SkipsValidation() {
	sw := repository.Switch{Type: "Linear"}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestValidFields_ReturnsNoErrors() {
	vendor := "Amazon"
	sw := repository.Switch{
		Type:     "Linear",
		Purchase: repository.SwitchPurchase{Vendor: &vendor},
	}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Empty(errs)
}

func (s *ValidateSwitchSuite) TestInvalidType_ReturnsFieldError() {
	sw := repository.Switch{Type: "NotAType"}

	errs := lookup.ValidateSwitch(s.T().Context(), sw)
	s.Equal([]lookup.FieldError{
		{Field: "type", Value: "NotAType", Category: lookup.CategorySwitchType},
	}, errs)
}
