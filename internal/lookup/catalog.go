package lookup

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// data holds one JSON file per category (e.g. data/vendor.json), named
// after the category itself, so an edit or diff to one category doesn't
// touch every other one.
//
//go:embed data
var data embed.FS

// Lookup is one lookup category's approved values, already decoded into
// their real per-category element type: []string for most categories, or
// []LayoutValue/[]CaseMountTypeValue for the categories whose entries are
// objects (see decodeValues). Values holds those typed elements widened to
// []any, not raw JSON (map[string]any) - a category's data is decoded once,
// at init(), not re-decoded by every caller.
type Lookup struct {
	Category Category
	Values   []any
}

// LayoutValue is one entry of the CategoryKeyboardLayout category.
type LayoutValue struct {
	Name  string   `json:"name"`
	Sizes []string `json:"sizes"`
}

// CaseMountTypeValue is one entry of the CategoryBuildCaseMountType
// category.
type CaseMountTypeValue struct {
	Name              string `json:"name"`
	SupportsDurometer bool   `json:"supports_durometer"`
}

// catalog is populated once in init() and read-only after that.
var catalog map[Category]Lookup

func init() {
	entries, err := data.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("internal/lookup: reading embedded data dir: %v", err))
	}

	catalog = make(map[Category]Lookup, len(entries))
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok {
			panic(fmt.Sprintf("internal/lookup: embedded data dir has non-.json file %q", entry.Name()))
		}
		category := Category(name)

		raw, err := data.ReadFile("data/" + entry.Name())
		if err != nil {
			panic(fmt.Sprintf("internal/lookup: reading embedded %s: %v", entry.Name(), err))
		}

		values, err := decodeValues(category, raw)
		if err != nil {
			panic(fmt.Sprintf("internal/lookup: data/%s.json has unexpected shape: %v", entry.Name(), err))
		}

		catalog[category] = Lookup{Category: category, Values: values}
	}

	validateInvariants()
}

// validateInvariants checks relationships that span more than one
// category's file. Must run after catalog is fully populated.
func validateInvariants() {
	validateLayoutSizesAreApproved()
}

// validateLayoutSizesAreApproved panics if any data/keyboard_layout.json
// entry's Sizes references a size not present in data/keyboard_size.json -
// see [validateKeyboardLayout], which relies on that relationship holding.
func validateLayoutSizesAreApproved() {
	layouts := catalog[CategoryKeyboardLayout].LayoutValues()
	approvedSizes := catalog[CategoryKeyboardSize].Strings()

	for _, l := range layouts {
		for _, size := range l.Sizes {
			if !slices.Contains(approvedSizes, size) {
				panic(fmt.Sprintf(
					"internal/lookup: data/keyboard_layout.json layout %q references size %q, which is not in data/keyboard_size.json",
					l.Name, size))
			}
		}
	}
}

// ListCategories returns every known category name, sorted. ctx is unused
// today (the catalog can't fail or block) but kept so a future caller
// doesn't need to change its call signature to add logging/tracing here.
func ListCategories(_ context.Context) []Category {
	return slices.Sorted(maps.Keys(catalog))
}

// ListCategoryNames is ListCategories widened to []string, for callers
// (e.g. REST/MCP list-lookups handlers) whose wire response is []string.
func ListCategoryNames(ctx context.Context) []string {
	categories := ListCategories(ctx)
	names := make([]string, len(categories))
	for i, c := range categories {
		names[i] = string(c)
	}
	return names
}

// GetCategory returns category's Lookup and true, or false if category
// isn't a known lookup category.
func GetCategory(_ context.Context, category Category) (Lookup, bool) {
	l, ok := catalog[category]
	if !ok {
		return Lookup{}, false
	}

	l.Values = slices.Clone(l.Values)
	return l, true
}

// Strings narrows l.Values to []string. Panics if l.Category's values
// aren't plain strings (see decodeValues) - same for LayoutValues and
// CaseMountTypeValues below, each for its own category.
func (l Lookup) Strings() []string { return typedValues[string](l.Values) }

// LayoutValues narrows l.Values to []LayoutValue.
func (l Lookup) LayoutValues() []LayoutValue { return typedValues[LayoutValue](l.Values) }

// CaseMountTypeValues narrows l.Values to []CaseMountTypeValue.
func (l Lookup) CaseMountTypeValues() []CaseMountTypeValue {
	return typedValues[CaseMountTypeValue](l.Values)
}

// typedValues type-asserts each element of values down to T, panicking on
// the first mismatch. Safe to call without an error return because
// decodeValues already decoded and shape-checked every category's values
// into its real per-category type at init() - a caller asking for the
// wrong T for a category is a programmer error, not a runtime condition.
func typedValues[T any](values []any) []T {
	out := make([]T, len(values))
	for i, v := range values {
		out[i] = v.(T) //nolint:forcetypeassert // see doc comment: decodeValues guarantees this.
	}
	return out
}

// decodeValues decodes raw (one category's embedded JSON file) into its
// real per-category element type - []LayoutValue for
// CategoryKeyboardLayout, []CaseMountTypeValue for
// CategoryBuildCaseMountType, []string otherwise - then widens the result
// to []any for storage in Lookup.Values. Called once per category at
// init(), so a shape mismatch in checked-in embedded data fails the build
// immediately, not a later request that happens to touch that category.
func decodeValues(category Category, raw []byte) ([]any, error) {
	switch category {
	case CategoryKeyboardLayout:
		return decodeNamedObjects(raw, func(v LayoutValue) string { return v.Name })
	case CategoryBuildCaseMountType:
		return decodeNamedObjects(raw, func(v CaseMountTypeValue) string { return v.Name })
	default:
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, fmt.Errorf("unmarshalling values: %w", err)
		}
		return toAnySlice(values), nil
	}
}

// decodeNamedObjects is decodeValues' object-shaped-category path:
// unmarshal raw as []T, then reject any entry with an empty name. name
// extracts the field to check, since T doesn't share a common Name field
// via any interface.
func decodeNamedObjects[T any](raw []byte, name func(T) string) ([]any, error) {
	var values []T
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("unmarshalling values: %w", err)
	}

	for i, v := range values {
		if name(v) == "" {
			return nil, fmt.Errorf("value at index %d missing required field %q", i, "name")
		}
	}

	return toAnySlice(values), nil
}

// toAnySlice widens a typed slice (e.g. []LayoutValue) to []any, the shape
// Lookup.Values holds so it can represent any category's element type.
func toAnySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}
