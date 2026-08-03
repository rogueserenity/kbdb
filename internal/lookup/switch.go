package lookup

import (
	"context"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ValidateSwitch returns every field on sw that isn't an approved value for
// its lookup category. An unset field is skipped, not treated as invalid.
func ValidateSwitch(ctx context.Context, repo repository.LookupRepository, sw repository.Switch) ([]FieldError, error) {
	var checks []fieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("type", &sw.Type, repository.CategorySwitchType)
	add("material.top_housing", sw.Material.TopHousing, repository.CategorySwitchMaterial)
	add("material.bottom_housing", sw.Material.BottomHousing, repository.CategorySwitchMaterial)
	add("material.stem", sw.Material.Stem, repository.CategorySwitchMaterial)
	add("spring.material", sw.Spring.Material, repository.CategorySwitchSpringMaterial)
	add("purchase.vendor", sw.Purchase.Vendor, repository.CategoryVendor)

	return validateFields(ctx, repo, checks)
}
