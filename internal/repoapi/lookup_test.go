package repoapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rogueserenity/kbdb/internal/repository"
)

func TestLookupToAPI_PlainStringCategory_PassesThrough(t *testing.T) {
	l := repository.Lookup{Category: "vendor", Values: []any{"Amazon", "CannonKeys"}}

	got, err := LookupToAPI(l)

	require.NoError(t, err)
	assert.Equal(t, "vendor", got.Category)
	assert.Equal(t, []any{"Amazon", "CannonKeys"}, got.Values)
}

func TestLookupToAPI_KeyboardLayout_DecodesTyped(t *testing.T) {
	l := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values: []any{
			map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}},
		},
	}

	got, err := LookupToAPI(l)

	require.NoError(t, err)
	require.Len(t, got.Values, 1)
	assert.Equal(t, repository.LayoutValue{Name: "WK", Sizes: []string{"60%", "65%"}}, got.Values[0])
}

func TestLookupToAPI_KeyboardLayout_WrongShape_Errors(t *testing.T) {
	l := repository.Lookup{
		Category: repository.CategoryKeyboardLayout,
		Values:   []any{"WK"},
	}

	_, err := LookupToAPI(l)

	require.Error(t, err)
}

func TestLookupToAPI_BuildCaseMountType_DecodesTyped(t *testing.T) {
	l := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values: []any{
			map[string]any{"name": "Gasket Mount", "supports_durometer": true},
		},
	}

	got, err := LookupToAPI(l)

	require.NoError(t, err)
	require.Len(t, got.Values, 1)
	assert.Equal(t, repository.CaseMountTypeValue{Name: "Gasket Mount", SupportsDurometer: true}, got.Values[0])
}

func TestLookupToAPI_BuildCaseMountType_WrongShape_Errors(t *testing.T) {
	l := repository.Lookup{
		Category: repository.CategoryBuildCaseMountType,
		Values:   []any{"Gasket Mount"},
	}

	_, err := LookupToAPI(l)

	require.Error(t, err)
}
