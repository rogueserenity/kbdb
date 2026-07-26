package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rogueserenity/kbdb/internal/repository"
)

func TestParseStrings(t *testing.T) {
	t.Run("all strings", func(t *testing.T) {
		got, err := repository.ParseStrings([]any{"a", "b"})
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, got)
	})

	t.Run("non-string entry errors", func(t *testing.T) {
		_, err := repository.ParseStrings([]any{"a", 1})
		require.Error(t, err)
	})
}

func TestParseLayoutValues(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := repository.ParseLayoutValues([]any{
			map[string]any{"name": "WK", "sizes": []any{"60%", "65%"}},
		})
		require.NoError(t, err)
		assert.Equal(t, []repository.LayoutValue{{Name: "WK", Sizes: []string{"60%", "65%"}}}, got)
	})

	t.Run("wrong shape errors", func(t *testing.T) {
		_, err := repository.ParseLayoutValues([]any{"WK"})
		require.Error(t, err)
	})

	t.Run("missing name errors", func(t *testing.T) {
		_, err := repository.ParseLayoutValues([]any{
			map[string]any{"sizes": []any{"60%"}},
		})
		require.Error(t, err)
	})
}

func TestParseCaseMountTypeValues(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := repository.ParseCaseMountTypeValues([]any{
			map[string]any{"name": "Gasket Mount", "supports_durometer": true},
		})
		require.NoError(t, err)
		assert.Equal(t, []repository.CaseMountTypeValue{{Name: "Gasket Mount", SupportsDurometer: true}}, got)
	})

	t.Run("wrong shape errors", func(t *testing.T) {
		_, err := repository.ParseCaseMountTypeValues([]any{"Gasket Mount"})
		require.Error(t, err)
	})

	t.Run("missing name errors", func(t *testing.T) {
		_, err := repository.ParseCaseMountTypeValues([]any{
			map[string]any{"supports_durometer": true},
		})
		require.Error(t, err)
	})
}
