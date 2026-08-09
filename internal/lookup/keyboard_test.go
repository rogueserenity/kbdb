package lookup_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/repository"
)

type ValidateKeyboardSuite struct {
	suite.Suite
}

func TestValidateKeyboardSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeyboardSuite))
}

func (s *ValidateKeyboardSuite) TestAllFieldsUnset_SkipsValidation() {
	kb := repository.Keyboard{}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_ReturnsFieldError() {
	size := "not-a-size"
	kb := repository.Keyboard{Size: &size}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "not-a-size", Category: lookup.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestValidSizeAndLayout_ReturnsNoErrors() {
	size := "60%"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Empty(errs)
}

func (s *ValidateKeyboardSuite) TestLayoutNotValidForSize_ReturnsFieldError() {
	size := "40%"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "WK", Category: lookup.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidSize_SkipsLayoutSizeCrossCheck() {
	size := "not-a-size"
	layoutName := "WK"
	kb := repository.Keyboard{Size: &size, Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "size", Value: "not-a-size", Category: lookup.CategoryKeyboardSize},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidLayoutName_ReturnsFieldError() {
	layoutName := "NotALayout"
	kb := repository.Keyboard{Layout: &layoutName}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "layout", Value: "NotALayout", Category: lookup.CategoryKeyboardLayout},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidPlateMaterial_ReturnsIndexedFieldError() {
	kb := repository.Keyboard{
		Design: repository.KeyboardDesign{Plates: []string{"AL", "NotAMaterial"}},
	}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "design.plates[1]", Value: "NotAMaterial", Category: lookup.CategoryKeyboardPlateMaterial},
	}, errs)
}
