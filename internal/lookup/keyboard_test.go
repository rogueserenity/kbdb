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

type ValidateKeyboardPartsSuite struct {
	suite.Suite
}

func TestValidateKeyboardPartsSuite(t *testing.T) {
	suite.Run(t, new(ValidateKeyboardPartsSuite))
}

func (s *ValidateKeyboardPartsSuite) TestValidPlate_ReturnsNoErrors() {
	s.Empty(lookup.ValidateKeyboardPlate(s.T().Context(), repository.KeyboardPlate{Material: "AL"}))
}

func (s *ValidateKeyboardPartsSuite) TestInvalidPlate_ReturnsFieldErrors() {
	bad := "NotAValue"
	plate := repository.KeyboardPlate{
		Material: "NotAMaterial",
		Purchase: repository.KeyboardPurchase{Vendor: &bad, OrderStatus: &bad},
	}

	errs := lookup.ValidateKeyboardPlate(s.T().Context(), plate)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "material", Value: "NotAMaterial", Category: lookup.CategoryKeyboardPlateMaterial},
		{Field: "purchase.vendor", Value: bad, Category: lookup.CategoryVendor},
		{Field: "purchase.order_status", Value: bad, Category: lookup.CategoryOrderStatus},
	}, errs)
}

func (s *ValidateKeyboardPartsSuite) TestUnsetPCBFields_SkipsValidation() {
	s.Empty(lookup.ValidateKeyboardPCB(s.T().Context(), repository.KeyboardPCB{}))
}

func (s *ValidateKeyboardPartsSuite) TestInvalidPCB_ReturnsFieldErrors() {
	bad := "NotAValue"
	pcb := repository.KeyboardPCB{
		Firmware: &bad, Assembly: &bad, Connectivity: &bad,
		Purchase: repository.KeyboardPurchase{OrderStatus: &bad},
	}

	errs := lookup.ValidateKeyboardPCB(s.T().Context(), pcb)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "firmware", Value: bad, Category: lookup.CategoryKeyboardPCBFirmware},
		{Field: "assembly", Value: bad, Category: lookup.CategoryKeyboardPCBAssemblyType},
		{Field: "connectivity", Value: bad, Category: lookup.CategoryKeyboardPCBConnectivityType},
		{Field: "purchase.order_status", Value: bad, Category: lookup.CategoryOrderStatus},
	}, errs)
}

func (s *ValidateKeyboardPartsSuite) TestValidPCB_ReturnsNoErrors() {
	v, a, c := "QMK/VIA", "EC", "Tri-Mode"
	s.Empty(lookup.ValidateKeyboardPCB(s.T().Context(), repository.KeyboardPCB{Firmware: &v, Assembly: &a, Connectivity: &c}))
}
