// Package repository defines, per entity, the data-model struct and the
// interface of operations available on it. No AWS/DynamoDB SDK types here —
// concrete implementations live in subpackages (e.g. internal/repository/dynamo).
package repository

import (
	"context"
	"encoding/json"
	"fmt"
)

// Lookup is a lookup category's approved values. values is usually a list
// of plain strings, but some categories store a list of objects instead —
// see model/lookup_seed.json.
type Lookup struct {
	Category string `dynamodbav:"category" json:"category"`
	Values   []any  `dynamodbav:"values" json:"values"`
}

// LookupRepository provides access to lookup categories.
type LookupRepository interface {
	ListCategories(ctx context.Context) ([]string, error)
	GetCategory(ctx context.Context, category string) (*Lookup, error)
	CreateCategory(ctx context.Context, lookup Lookup) (*Lookup, error)
	ReplaceCategory(ctx context.Context, lookup Lookup) (*Lookup, error)
	DeleteCategory(ctx context.Context, category string) error
}

// LayoutValue is one entry of the CategoryKeyboardLayout category.
type LayoutValue struct {
	Name  string   `json:"name"`
	Sizes []string `json:"sizes"`
}

// CaseMountTypeValue is one entry of the CategoryBuildCaseMountType category.
type CaseMountTypeValue struct {
	Name              string `json:"name"`
	SupportsDurometer bool   `json:"supports_durometer"`
}

// Strings errors on a non-string entry instead of skipping it: that means
// the category's data isn't shaped the way a caller expects, not that a
// value is merely unapproved.
func (l Lookup) Strings() ([]string, error) {
	out := make([]string, 0, len(l.Values))
	for i, v := range l.Values {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("value at index %d is not a string: %#v", i, v)
		}
		out = append(out, s)
	}

	return out, nil
}

func (l Lookup) LayoutValues() ([]LayoutValue, error) {
	out, err := parseObjects[LayoutValue](l.Values)
	if err != nil {
		return nil, err
	}

	for i, v := range out {
		if v.Name == "" {
			return nil, fmt.Errorf("value at index %d missing required field %q", i, "name")
		}
	}

	return out, nil
}

func (l Lookup) CaseMountTypeValues() ([]CaseMountTypeValue, error) {
	out, err := parseObjects[CaseMountTypeValue](l.Values)
	if err != nil {
		return nil, err
	}

	for i, v := range out {
		if v.Name == "" {
			return nil, fmt.Errorf("value at index %d missing required field %q", i, "name")
		}
	}

	return out, nil
}

// parseObjects round-trips through encoding/json since DynamoDB unmarshals
// object entries as map[string]any.
func parseObjects[T any](values []any) ([]T, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshalling values: %w", err)
	}

	out := make([]T, 0, len(values))
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("unmarshalling values: %w", err)
	}

	return out, nil
}
