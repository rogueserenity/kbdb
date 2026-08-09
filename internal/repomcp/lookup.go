package repomcp

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
)

// LookupToMCP decodes CategoryKeyboardLayout/CategoryBuildCaseMountType into
// their typed shape first, so a mismatch in stored data errors here instead
// of silently returning whatever was actually stored. l must come from
// lookup.GetCategory - see LookupToAPI's doc comment for why.
func LookupToMCP(l lookup.Lookup) (schema.GetLookupOutput, error) {
	values := l.Values

	switch l.Category {
	case lookup.CategoryKeyboardLayout:
		layouts, err := l.LayoutValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = lookup.ToAnySlice(layouts)
	case lookup.CategoryBuildCaseMountType:
		mountTypes, err := l.CaseMountTypeValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = lookup.ToAnySlice(mountTypes)
	}

	return schema.GetLookupOutput{
		Category: string(l.Category),
		Values:   values,
	}, nil
}
