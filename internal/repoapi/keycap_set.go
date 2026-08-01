package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToAPI maps a repository.KeycapSet to its wire representation.
func KeycapSetToAPI(ks repository.KeycapSet) api.KeycapSet {
	return api.KeycapSet{
		Id:         ks.ID,
		Brand:      ks.Brand,
		Name:       ks.Name,
		Profile:    ks.Profile,
		Material:   ks.Material,
		Notes:      ks.Notes,
		Visibility: api.Visibility(ks.Visibility),
	}
}

// KeycapSetToRepo maps a generated KeycapSetInput (already schema-validated
// by the OpenAPI request validator) to a repository.KeycapSet. It does not
// set UserID or ID - those come from the request's path/caller, not the
// body, and stay the handler's responsibility.
func KeycapSetToRepo(in api.KeycapSetInput) repository.KeycapSet {
	return repository.KeycapSet{
		Brand:      in.Brand,
		Name:       in.Name,
		Profile:    in.Profile,
		Material:   in.Material,
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// KeycapSetToAPISummary maps a repository.KeycapSet to the
// KeycapSetSummary schema returned by the list endpoint.
func KeycapSetToAPISummary(ks repository.KeycapSet) api.KeycapSetSummary {
	return api.KeycapSetSummary{
		Id:      &ks.ID,
		Brand:   &ks.Brand,
		Name:    &ks.Name,
		Profile: ks.Profile,
	}
}
