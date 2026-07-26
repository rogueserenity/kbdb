package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// LookupToAPI maps a repository.Lookup to its wire representation.
func LookupToAPI(l repository.Lookup) api.Lookup {
	return api.Lookup{
		Category: l.Category,
		Values:   l.Values,
	}
}
