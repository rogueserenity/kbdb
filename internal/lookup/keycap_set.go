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
