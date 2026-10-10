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
		Plates: []repository.KeyboardPlate{{Material: "AL"}, {Material: "NotAMaterial"}},
	}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.Equal([]lookup.FieldError{
		{Field: "plates[1].material", Value: "NotAMaterial", Category: lookup.CategoryKeyboardPlateMaterial},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidPCBFields_ReturnsIndexedFieldErrors() {
	valid := "QMK/VIA"
	bad := "NotAValue"
	kb := repository.Keyboard{
		PCBs: []repository.KeyboardPCB{
			{Firmware: &valid},
			{Firmware: &bad, Assembly: &bad, Connectivity: &bad},
		},
	}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "pcbs[1].firmware", Value: bad, Category: lookup.CategoryKeyboardPCBFirmware},
		{Field: "pcbs[1].assembly", Value: bad, Category: lookup.CategoryKeyboardPCBAssemblyType},
		{Field: "pcbs[1].connectivity", Value: bad, Category: lookup.CategoryKeyboardPCBConnectivityType},
	}, errs)
}

func (s *ValidateKeyboardSuite) TestInvalidPartPurchase_ReturnsIndexedFieldErrors() {
	bad := "NotAValue"
	kb := repository.Keyboard{
		Plates: []repository.KeyboardPlate{{Material: "AL", Purchase: repository.KeyboardPurchase{Vendor: &bad}}},
		PCBs:   []repository.KeyboardPCB{{Purchase: repository.KeyboardPurchase{OrderStatus: &bad}}},
	}

	errs := lookup.ValidateKeyboard(s.T().Context(), kb)
	s.ElementsMatch([]lookup.FieldError{
		{Field: "plates[0].purchase.vendor", Value: bad, Category: lookup.CategoryVendor},
		{Field: "pcbs[0].purchase.order_status", Value: bad, Category: lookup.CategoryOrderStatus},
	}, errs)
}
