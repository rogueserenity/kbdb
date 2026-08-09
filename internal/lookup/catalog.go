package lookup

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// data holds one JSON file per category (e.g. data/vendor.json), named
// after the category itself, so an edit or diff to one category doesn't
// touch every other one.
//
//go:embed data
var data embed.FS

// Lookup is one lookup category's approved values. Values is usually a
// list of plain strings, but some categories store a list of objects
// instead - see LayoutValue, CaseMountTypeValue.
type Lookup struct {
	Category Category `json:"category"`
	Values   []any    `json:"values"`
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

// stringsUncached errors on a non-string entry instead of skipping it: that
// means the category's data isn't shaped the way a caller expects, not
// that a value is merely unapproved.
func (l Lookup) stringsUncached() ([]string, error) {
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

func (l Lookup) layoutValuesUncached() ([]LayoutValue, error) {
	return parseNamedObjects(l.Values, func(v LayoutValue) string { return v.Name })
}

func (l Lookup) caseMountTypeValuesUncached() ([]CaseMountTypeValue, error) {
	return parseNamedObjects(l.Values, func(v CaseMountTypeValue) string { return v.Name })
}

// parseNamedObjects is parseObjects plus rejecting an entry with an empty
// name. name extracts the field to check, since T doesn't share a common
// Name field via any interface.
func parseNamedObjects[T any](values []any, name func(T) string) ([]T, error) {
	out, err := parseObjects[T](values)
	if err != nil {
		return nil, err
	}

	for i, v := range out {
		if name(v) == "" {
			return nil, fmt.Errorf("value at index %d missing required field %q", i, "name")
		}
	}

	return out, nil
}

// Strings returns l.Values decoded as a flat string slice. l must come
// from GetCategory: this decodes by l.Category against the catalog, not
// l.Values itself, so a hand-built Lookup with fabricated Values decodes
// as whatever the real category's data is.
func (l Lookup) Strings() ([]string, error) {
	return decoded[[]string](l.Category)
}

// LayoutValues is Strings for []LayoutValue.
func (l Lookup) LayoutValues() ([]LayoutValue, error) {
	return decoded[[]LayoutValue](l.Category)
}

// CaseMountTypeValues is Strings for []CaseMountTypeValue.
func (l Lookup) CaseMountTypeValues() ([]CaseMountTypeValue, error) {
	return decoded[[]CaseMountTypeValue](l.Category)
}

// ToAnySlice widens a typed slice (e.g. []LayoutValue) back to []any, the
// shape Lookup.Values and the wire formats built on top of it use.
func ToAnySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}

// parseObjects round-trips through encoding/json since a category's object
// entries decode from JSON as map[string]any.
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

// catalog, decodedValues, categoryNames, and categoryNameStrings are all
// populated once in init() and read-only after that. decodedValues holds
// each category's Values pre-decoded into its expected typed shape
// ([]string, []LayoutValue, or []CaseMountTypeValue); categoryNames/
// categoryNameStrings are the sorted category list in each form. Exported
// accessors below return clones, never these directly.
var (
	catalog             map[Category]Lookup
	decodedValues       map[Category]any
	categoryNames       []Category
	categoryNameStrings []string
)

func init() {
	entries, err := data.ReadDir("data")
	if err != nil {
		panic(fmt.Sprintf("internal/lookup: reading embedded data dir: %v", err))
	}

	catalog = make(map[Category]Lookup, len(entries))
	decodedValues = make(map[Category]any, len(entries))
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

		var values []any
		if err := json.Unmarshal(raw, &values); err != nil {
			panic(fmt.Sprintf("internal/lookup: parsing embedded %s: %v", entry.Name(), err))
		}

		l := Lookup{Category: category, Values: values}
		validateShape(l)
		catalog[category] = l
	}

	validateInvariants()

	categoryNames = make([]Category, 0, len(catalog))
	for name := range catalog {
		categoryNames = append(categoryNames, name)
	}
	slices.Sort(categoryNames)

	categoryNameStrings = make([]string, len(categoryNames))
	for i, name := range categoryNames {
		categoryNameStrings[i] = string(name)
	}
}

// validateShape decodes l's values via whichever typed accessor its
// category expects and caches the result into decodedValues, panicking if
// decoding fails - a shape mismatch in checked-in embedded data should
// fail the build, not surface as a panic against a live request later.
func validateShape(l Lookup) {
	var err error
	switch l.Category {
	case CategoryKeyboardLayout:
		decodedValues[l.Category], err = l.layoutValuesUncached()
	case CategoryBuildCaseMountType:
		decodedValues[l.Category], err = l.caseMountTypeValuesUncached()
	default:
		decodedValues[l.Category], err = l.stringsUncached()
	}
	if err != nil {
		panic(fmt.Sprintf("internal/lookup: data/%s.json has unexpected shape: %v", l.Category, err))
	}
}

// decoded returns a clone of category's pre-decoded value from
// decodedValues, type-asserted to T. Errors if category is unknown or was
// decoded to a different type than T (the wrong accessor for that
// category's shape).
func decoded[T ~[]E, E any](category Category) (T, error) {
	var zero T

	v, ok := decodedValues[category]
	if !ok {
		return zero, fmt.Errorf("no decoded value cached for category %q", category)
	}

	t, ok := v.(T)
	if !ok {
		return zero, fmt.Errorf("category %q decoded as %T, not %T", category, v, zero)
	}

	return slices.Clone(t), nil
}

// validateInvariants checks relationships that span more than one
// category's file. Must run after catalog is fully populated.
func validateInvariants() {
	validateLayoutSizesAreApproved()
}

// validateLayoutSizesAreApproved panics if any data/keyboard_layout.json
// entry's Sizes references a size not present in data/keyboard_size.json -
// see internal/lookup/keyboard.go's validateKeyboardLayout, which relies
// on that relationship holding.
func validateLayoutSizesAreApproved() {
	layouts, err := catalog[CategoryKeyboardLayout].LayoutValues()
	if err != nil {
		panic(fmt.Sprintf("internal/lookup: %v", err))
	}

	approvedSizes, err := catalog[CategoryKeyboardSize].Strings()
	if err != nil {
		panic(fmt.Sprintf("internal/lookup: %v", err))
	}

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
	return slices.Clone(categoryNames)
}

// ListCategoryNames is ListCategories widened to []string, for callers
// (e.g. REST/MCP list-lookups handlers) whose wire response is []string.
func ListCategoryNames(_ context.Context) []string {
	return slices.Clone(categoryNameStrings)
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
