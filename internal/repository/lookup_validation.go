package repository

import (
	"context"
	"errors"
	"slices"
)

// FieldCheck is one entity field to validate against a lookup category.
type FieldCheck struct {
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

// ValidateFields fetches each distinct category at most once and returns
// every invalid field, not just the first.
func ValidateFields(ctx context.Context, repo LookupRepository, checks []FieldCheck) ([]FieldError, error) {
	values := make(map[string][]string, len(checks))

	for _, c := range checks {
		if _, ok := values[c.Category]; ok {
			continue
		}

		lookup, err := repo.GetCategory(ctx, c.Category)
		if errors.Is(err, ErrNotFound) {
			values[c.Category] = nil
			continue
		}
		if err != nil {
			return nil, err
		}

		strs, err := ParseStrings(lookup.Values)
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
