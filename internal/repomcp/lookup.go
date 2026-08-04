package repomcp

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// LookupToMCP decodes CategoryKeyboardLayout/CategoryBuildCaseMountType into
// their typed shape first, so a mismatch in stored data errors here instead
// of silently returning whatever was actually stored.
func LookupToMCP(l repository.Lookup) (schema.GetLookupOutput, error) {
	values := l.Values

	switch l.Category {
	case repository.CategoryKeyboardLayout:
		layouts, err := l.LayoutValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(layouts)
	case repository.CategoryBuildCaseMountType:
		mountTypes, err := l.CaseMountTypeValues()
		if err != nil {
			return schema.GetLookupOutput{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(mountTypes)
	}

	return schema.GetLookupOutput{
		Category: l.Category,
		Values:   values,
	}, nil
}

func toAnySlice[T any](items []T) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}

	return out
}
