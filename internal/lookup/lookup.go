// Package lookup validates entity fields against approved lookup category
// values, independent of any transport (REST, MCP). It's the one place
// that knows which fields on which entities are open-vocabulary and which
// lookup category each maps to; internal/repository stays data-access-only
// with no validation logic of its own.
package lookup

import (
	"context"
	"errors"
	"slices"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// fieldCheck is one entity field to validate against a lookup category.
type fieldCheck struct {
	Field    string
	Value    string
	Category string
}

// FieldError reports that Field's Value isn't an approved Category value.
type FieldError struct {
	Field    string
	Value    string
	Category string
}

// ValidateImageContentType reports whether contentType is an approved
// CategoryImageContentType value. It returns a FieldError, not an error,
// when it isn't - that's an expected validation outcome, not a failure.
func ValidateImageContentType(ctx context.Context, repo repository.LookupRepository, contentType string) (*FieldError, error) {
	errs, err := validateFields(ctx, repo, []fieldCheck{
		{Field: "content_type", Value: contentType, Category: repository.CategoryImageContentType},
	})
	if err != nil {
		return nil, err
	}
	if len(errs) == 0 {
		return nil, nil //nolint:nilnil // no problem found is a valid, expected result
	}
	return &errs[0], nil
}

// validateFields fetches each distinct category at most once and returns
// every invalid field, not just the first.
func validateFields(ctx context.Context, repo repository.LookupRepository, checks []fieldCheck) ([]FieldError, error) {
	values := make(map[string][]string, len(checks))

	for _, c := range checks {
		if _, ok := values[c.Category]; ok {
			continue
		}

		lookup, err := repo.GetCategory(ctx, c.Category)
		if errors.Is(err, repository.ErrNotFound) {
			values[c.Category] = nil
			continue
		}
		if err != nil {
			return nil, err
		}

		strs, err := lookup.Strings()
		if err != nil {
			return nil, err
		}
		values[c.Category] = strs
	}

	var errs []FieldError
	for _, c := range checks {
		if !slices.Contains(values[c.Category], c.Value) {
			errs = append(errs, FieldError(c))
		}
	}

	return errs, nil
}
