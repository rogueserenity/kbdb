package lookup

import (
	"context"
	"slices"
)

// fieldCheck is one entity field to validate against a lookup category.
type fieldCheck struct {
	Field    string
	Value    string
	Category Category
}

// FieldError reports that Field's Value isn't an approved Category value.
type FieldError struct {
	Field    string
	Value    string
	Category Category
}

// ValidateImageContentType reports whether contentType is an approved
// CategoryImageContentType value. It returns a FieldError, not an error,
// when it isn't - that's an expected validation outcome, not a failure.
func ValidateImageContentType(ctx context.Context, contentType string) *FieldError {
	errs := validateFields(ctx, []fieldCheck{
		{Field: "content_type", Value: contentType, Category: CategoryImageContentType},
	})
	if len(errs) == 0 {
		return nil
	}
	return &errs[0]
}

// validateFields fetches each distinct category at most once and returns
// every invalid field, not just the first.
func validateFields(ctx context.Context, checks []fieldCheck) []FieldError {
	values := make(map[Category][]string, len(checks))

	for _, c := range checks {
		if _, ok := values[c.Category]; ok {
			continue
		}

		l, ok := GetCategory(ctx, c.Category)
		if !ok {
			values[c.Category] = nil
			continue
		}

		strs, err := l.Strings()
		if err != nil {
			// Only reachable if a fieldCheck names an object-shaped
			// category (e.g. keyboard_layout) instead of routing through a
			// dedicated caller like validateKeyboardLayout - a caller bug.
			panic(err)
		}
		values[c.Category] = strs
	}

	var errs []FieldError
	for _, c := range checks {
		if !slices.Contains(values[c.Category], c.Value) {
			errs = append(errs, FieldError(c))
		}
	}

	return errs
}
