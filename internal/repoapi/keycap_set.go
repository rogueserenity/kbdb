package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

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
