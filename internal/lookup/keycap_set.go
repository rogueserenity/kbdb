package lookup

import (
	"context"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ValidateKeycapSet returns every field on ks that isn't an approved value
// for its lookup category. An unset field is skipped, not treated as
// invalid.
func ValidateKeycapSet(ctx context.Context, repo repository.LookupRepository, ks repository.KeycapSet) ([]FieldError, error) {
	var checks []fieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("profile", ks.Profile, repository.CategoryKeycapProfile)
	add("material", ks.Material, repository.CategoryKeycapMaterial)

	return validateFields(ctx, repo, checks)
}

// ValidateKeycapKit returns every field on k that isn't an approved value
// for its lookup category. An unset field is skipped, not treated as
// invalid. Mirrors ValidateSwitch/ValidateKeyboard's purchase.vendor and
// purchase.order_status checks - a kit's purchase is structurally the same
// shape as a keyboard's or switch's.
func ValidateKeycapKit(ctx context.Context, repo repository.LookupRepository, k repository.KeycapKit) ([]FieldError, error) {
	var checks []fieldCheck
	add := func(field string, value *string, category string) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	add("purchase.vendor", k.Purchase.Vendor, repository.CategoryVendor)
	add("purchase.order_status", k.Purchase.OrderStatus, repository.CategoryOrderStatus)

	return validateFields(ctx, repo, checks)
}
