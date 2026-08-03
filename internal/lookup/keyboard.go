package lookup

import (
	"context"
	"errors"
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
func ValidateKeyboard(ctx context.Context, repo repository.LookupRepository, kb repository.Keyboard) ([]FieldError, error) {
	var checks []fieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("size", kb.Size, repository.CategoryKeyboardSize)
	add("design.top_case.material", kb.Design.TopCase.Material, repository.CategoryKeyboardCaseMaterial)
	add("design.bottom_case.material", kb.Design.BottomCase.Material, repository.CategoryKeyboardCaseMaterial)
	add("design.weight.material", kb.Design.Weight.Material, repository.CategoryKeyboardWeightMaterial)
	add("pcb.firmware", kb.PCB.Firmware, repository.CategoryKeyboardPCBFirmware)
	add("pcb.assembly", kb.PCB.Assembly, repository.CategoryKeyboardPCBAssemblyType)
	add("pcb.connectivity", kb.PCB.Connectivity, repository.CategoryKeyboardPCBConnectivityType)
	add("purchase.vendor", kb.Purchase.Vendor, repository.CategoryVendor)
	add("purchase.order_status", kb.Purchase.OrderStatus, repository.CategoryOrderStatus)

	for i, material := range kb.Design.Plates {
		checks = append(checks, fieldCheck{
			Field:    fmt.Sprintf("design.plates[%d]", i),
			Value:    material,
			Category: repository.CategoryKeyboardPlateMaterial,
		})
	}

	fieldErrs, err := validateFields(ctx, repo, checks)
	if err != nil {
		return nil, err
	}

	if kb.Layout == nil {
		return fieldErrs, nil
	}

	sizeInvalid := slices.ContainsFunc(fieldErrs, func(fe FieldError) bool { return fe.Field == "size" })
	size := kb.Size
	if sizeInvalid {
		size = nil
	}

	layoutErr, err := validateKeyboardLayout(ctx, repo, size, *kb.Layout)
	if err != nil {
		return nil, err
	}
	if layoutErr != nil {
		fieldErrs = append(fieldErrs, *layoutErr)
	}

	return fieldErrs, nil
}

// validateKeyboardLayout also cross-checks layout's sizes against
// CategoryKeyboardSize, so a keyboard can't pass its layout-vs-size check
// against a size that was never itself approved.
func validateKeyboardLayout(ctx context.Context, repo repository.LookupRepository, size *string, layout string) (*FieldError, error) {
	category := repository.CategoryKeyboardLayout

	lookupCategory, err := repo.GetCategory(ctx, category)
	if errors.Is(err, repository.ErrNotFound) {
		return &FieldError{Field: "layout", Value: layout, Category: category}, nil
	}
	if err != nil {
		return nil, err
	}

	values, err := lookupCategory.LayoutValues()
	if err != nil {
		return nil, err
	}

	idx := slices.IndexFunc(values, func(v repository.LayoutValue) bool { return v.Name == layout })
	if idx == -1 {
		return &FieldError{Field: "layout", Value: layout, Category: category}, nil
	}

	if size != nil && !slices.Contains(values[idx].Sizes, *size) {
		return &FieldError{Field: "layout", Value: layout, Category: repository.CategoryKeyboardSize}, nil
	}

	return nil, nil //nolint:nilnil // no problem found is a valid, expected result
}
