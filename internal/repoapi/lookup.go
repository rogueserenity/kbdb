package repoapi

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// LookupToAPI maps a repository.Lookup to its wire representation, decoding
// CategoryKeyboardLayout/CategoryBuildCaseMountType into their typed shape
// first so a mismatch in stored data errors here instead of silently
// serializing whatever was actually stored.
func LookupToAPI(l repository.Lookup) (api.Lookup, error) {
	values := l.Values

	switch l.Category {
	case repository.CategoryKeyboardLayout:
		layouts, err := repository.ParseLayoutValues(l.Values)
		if err != nil {
			return api.Lookup{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(layouts)
	case repository.CategoryBuildCaseMountType:
		mountTypes, err := repository.ParseCaseMountTypeValues(l.Values)
		if err != nil {
			return api.Lookup{}, fmt.Errorf("decoding %s values: %w", l.Category, err)
		}
		values = toAnySlice(mountTypes)
	}

	return api.Lookup{
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

// LookupInputToRepo maps a generated LookupInput (already schema-validated
// by the OpenAPI request validator) to the values slice
// LookupRepository.CreateCategory/ReplaceCategory take. There's no
// LookupInputToRepo equivalent of repository.Lookup itself - Category is a
// path parameter, not part of the request body, so the repo layer's write
// methods take values alone rather than a full Lookup.
func LookupInputToRepo(in api.LookupInput) []any {
	return in.Values
}
