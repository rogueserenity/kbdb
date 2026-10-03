package lookup

import (
	"context"
	"slices"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// ValidateBuild does not check whether b.Keyboard/Switches/KeycapKits
// reference entities that actually exist - that needs live repository
// data, not this package's static lookup-category model, so it's handled
// separately by
// [github.com/rogueserenity/kbdb/internal/buildrefs.ValidateReferences].
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
// mirrors [validateKeyboardLayout]'s handling of [CategoryKeyboardLayout]).
// An approved case_mount_type.durometer is also cross-checked against the
// type, the way layout is cross-checked against size: unless the type is
// set and supports_durometer, it's reported as a FieldError on
// "case_mount_type.durometer" carrying CategoryBuildCaseMountType. An
// unapproved type skips the cross-check, having already been reported.
func validateBuildCaseMountType(ctx context.Context, cmt repository.BuildCaseMountType) []FieldError {
	var errs []FieldError

	var mountType *CaseMountTypeValue
	if cmt.Type != nil {
		category := CategoryBuildCaseMountType
		l, ok := GetCategory(ctx, category)
		var values []CaseMountTypeValue
		if ok {
			values = l.CaseMountTypeValues()
		}
		idx := slices.IndexFunc(values, func(v CaseMountTypeValue) bool { return v.Name == *cmt.Type })
		if idx == -1 {
			errs = append(errs, FieldError{Field: "case_mount_type.type", Value: *cmt.Type, Category: category})
		} else {
			mountType = &values[idx]
		}
	}

	if cmt.Durometer != nil {
		durometerErrs := validateFields(ctx, []fieldCheck{
			{Field: "case_mount_type.durometer", Value: *cmt.Durometer, Category: CategoryBuildDurometer},
		})
		if len(durometerErrs) == 0 && (cmt.Type == nil || mountType != nil && !mountType.SupportsDurometer) {
			durometerErrs = append(durometerErrs, FieldError{Field: "case_mount_type.durometer", Value: *cmt.Durometer, Category: CategoryBuildCaseMountType})
		}
		errs = append(errs, durometerErrs...)
	}

	return errs
}
