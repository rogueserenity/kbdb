package lookup

import (
	"context"
	"slices"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ValidateBuild returns every field on b that isn't an approved value for
// its lookup category. An unset field is skipped, not treated as invalid.
// Does not check whether b.Keyboard/Switches/KeycapKits reference entities
// that actually exist and are owned by the caller - that needs
// KeyboardRepository/SwitchRepository/KeycapSetRepository in addition to
// b itself, which doesn't fit this package's static lookup-category model,
// so it's handled separately by internal/buildrefs.ValidateReferences,
// called from the REST/MCP handler layer alongside this function rather
// than folded into it.
func ValidateBuild(ctx context.Context, b repository.Build) []FieldError {
	var checks []fieldCheck
	add := func(field string, value *string, category Category) {
		if value == nil {
			return
		}
		checks = append(checks, fieldCheck{Field: field, Value: *value, Category: category})
	}

	if b.Stabs != nil {
		add("stabs.name", b.Stabs.Name, CategoryBuildStabilizer)
		add("stabs.mount_type", b.Stabs.MountType, CategoryBuildStabilizerMountType)
	}

	fieldErrs := validateFields(ctx, checks)

	if b.CaseMountType != nil {
		fieldErrs = append(fieldErrs, validateBuildCaseMountType(ctx, *b.CaseMountType)...)
	}

	return fieldErrs
}

// validateBuildCaseMountType checks case_mount_type.type against
// CategoryBuildCaseMountType's named-object entries (not a plain string
// list, so it can't go through the generic fieldCheck/validateFields path -
// mirrors validateKeyboardLayout's handling of CategoryKeyboardLayout).
// case_mount_type.durometer is still validated as a plain string against
// CategoryBuildDurometer regardless of whether the chosen type's
// supports_durometer is true - api/openapi.yaml documents durometer as
// "only meaningful" when supported, not "rejected" otherwise, so an unused
// durometer value is still checked for validity, just not cross-checked
// against the type the way layout is cross-checked against size.
func validateBuildCaseMountType(ctx context.Context, cmt repository.BuildCaseMountType) []FieldError {
	var errs []FieldError

	if cmt.Type != nil {
		category := CategoryBuildCaseMountType
		l, ok := GetCategory(ctx, category)
		if !ok || !slices.ContainsFunc(l.CaseMountTypeValues(), func(v CaseMountTypeValue) bool { return v.Name == *cmt.Type }) {
			errs = append(errs, FieldError{Field: "case_mount_type.type", Value: *cmt.Type, Category: category})
		}
	}

	if cmt.Durometer != nil {
		errs = append(errs, validateFields(ctx, []fieldCheck{
			{Field: "case_mount_type.durometer", Value: *cmt.Durometer, Category: CategoryBuildDurometer},
		})...)
	}

	return errs
}
