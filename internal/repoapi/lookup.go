package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/lookup"
)

// Lookup maps lookup.Lookup to its wire representation. It has no
// dependencies - lookups are static, deploy-time data.
type Lookup struct{}

// ToAPI maps a lookup.Lookup to its wire representation.
func (Lookup) ToAPI(l lookup.Lookup) api.Lookup {
	return api.Lookup{
		Category: string(l.Category),
		Values:   l.Values,
	}
}
