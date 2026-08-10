package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/lookup"
)

// LookupToAPI maps a lookup.Lookup to its wire representation.
func LookupToAPI(l lookup.Lookup) api.Lookup {
	return api.Lookup{
		Category: string(l.Category),
		Values:   l.Values,
	}
}
