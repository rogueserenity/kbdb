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
