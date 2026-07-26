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

// LookupInputToRepo maps a generated LookupInput (already schema-validated
// by the OpenAPI request validator) to the values slice
// LookupRepository.CreateCategory/ReplaceCategory take. There's no
// LookupInputToRepo equivalent of repository.Lookup itself - Category is a
// path parameter, not part of the request body, so the repo layer's write
// methods take values alone rather than a full Lookup.
func LookupInputToRepo(in api.LookupInput) []any {
	return in.Values
}
