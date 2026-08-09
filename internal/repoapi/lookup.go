package repoapi

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/lookup"
)

// LookupToAPI maps a lookup.Lookup to its wire representation, decoding
// CategoryKeyboardLayout/CategoryBuildCaseMountType into their typed shape
// first so a mismatch in stored data errors here instead of silently
// serializing whatever was actually stored. l must come from
// lookup.GetCategory - LayoutValues/CaseMountTypeValues read from a cache
// keyed by l.Category, populated once at package init from the real
// catalog data, not from l.Values itself, so a hand-built Lookup with
// fabricated Values decodes as the real category's data instead.
func LookupToAPI(l lookup.Lookup) (api.Lookup, error) {
	values := l.Values

	switch l.Category {
	case lookup.CategoryKeyboardLayout:
		layouts, err := l.LayoutValues()
		if err != nil {
			return api.Lookup{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = lookup.ToAnySlice(layouts)
	case lookup.CategoryBuildCaseMountType:
		mountTypes, err := l.CaseMountTypeValues()
		if err != nil {
			return api.Lookup{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = lookup.ToAnySlice(mountTypes)
	}

	return api.Lookup{
		Category: string(l.Category),
		Values:   values,
	}, nil
}
