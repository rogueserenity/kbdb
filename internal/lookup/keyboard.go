package lookup

import (
	"context"
	"fmt"
	"slices"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ValidateKeyboard returns every field on kb that isn't an approved value
// for its lookup category. An unset field is skipped, not treated as
// invalid.
//
// An already-invalid size is never cross-checked against Layout's approved
// sizes: that check would always fail too, reporting a second, misleading
// error blaming a perfectly valid layout instead of the real problem (size
// itself).
func ValidateKeyboard(ctx context.Context, kb repository.Keyboard) []FieldError {
	var checks []fieldCheck
	add := func(field string, value *string, category Category) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("size", kb.Size, CategoryKeyboardSize)
	add("design.top_case.material", kb.Design.TopCase.Material, CategoryKeyboardCaseMaterial)
	add("design.bottom_case.material", kb.Design.BottomCase.Material, CategoryKeyboardCaseMaterial)
	add("design.weight.material", kb.Design.Weight.Material, CategoryKeyboardWeightMaterial)
	add("pcb.firmware", kb.PCB.Firmware, CategoryKeyboardPCBFirmware)
	add("pcb.assembly", kb.PCB.Assembly, CategoryKeyboardPCBAssemblyType)
	add("pcb.connectivity", kb.PCB.Connectivity, CategoryKeyboardPCBConnectivityType)
	add("purchase.vendor", kb.Purchase.Vendor, CategoryVendor)
	add("purchase.order_status", kb.Purchase.OrderStatus, CategoryOrderStatus)

	for i, material := range kb.Design.Plates {
		checks = append(checks, fieldCheck{
			Field:    fmt.Sprintf("design.plates[%d]", i),
			Value:    material,
			Category: CategoryKeyboardPlateMaterial,
		})
	}

	fieldErrs := validateFields(ctx, checks)

	if kb.Layout == nil {
		return fieldErrs
	}

	sizeInvalid := slices.ContainsFunc(fieldErrs, func(fe FieldError) bool { return fe.Field == "size" })
	size := kb.Size
	if sizeInvalid {
		size = nil
	}

	if layoutErr := validateKeyboardLayout(ctx, size, *kb.Layout); layoutErr != nil {
		fieldErrs = append(fieldErrs, *layoutErr)
	}

	return fieldErrs
}

// validateKeyboardLayout also cross-checks layout's sizes against
// CategoryKeyboardSize, so a keyboard can't pass its layout-vs-size check
// against a size that was never itself approved.
func validateKeyboardLayout(ctx context.Context, size *string, layout string) *FieldError {
	category := CategoryKeyboardLayout

	l, ok := GetCategory(ctx, category)
	if !ok {
		return &FieldError{Field: "layout", Value: layout, Category: category}
	}

	values := l.LayoutValues()

	idx := slices.IndexFunc(values, func(v LayoutValue) bool { return v.Name == layout })
	if idx == -1 {
		return &FieldError{Field: "layout", Value: layout, Category: category}
	}

	if size != nil && !slices.Contains(values[idx].Sizes, *size) {
		return &FieldError{Field: "layout", Value: layout, Category: CategoryKeyboardSize}
	}

	return nil
}
