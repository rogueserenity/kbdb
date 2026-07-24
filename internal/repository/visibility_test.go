package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rogueserenity/kbdb/internal/repository"
)

func TestVisibility_Valid(t *testing.T) {
	tests := []struct {
		name string
		v    repository.Visibility
		want bool
	}{
		{"public", repository.VisibilityPublic, true},
		{"authenticated", repository.VisibilityAuthenticated, true},
		{"private", repository.VisibilityPrivate, true},
		{"empty", repository.Visibility(""), false},
		{"unknown", repository.Visibility("shared"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.v.Valid())
		})
	}
}
