package repoapi

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileToAPI maps a repository.Profile to its wire representation. It
// presigns the avatar if one is set, and never emits the owner's IdP
// subject. Returns an error only if presigning fails.
func ProfileToAPI(ctx context.Context, p repository.Profile, images repository.ProfileImageStore) (api.Profile, error) {
	out := api.Profile{
		Username:        p.Username,
		Discoverable:    &p.Discoverable,
		DiscordUsername: p.DiscordUsername,
		Bio:             p.Bio,
		Links:           profileLinksToAPI(p.Links),
	}

	if p.AvatarPath != nil {
		url, err := images.PresignGet(ctx, *p.AvatarPath)
		if err != nil {
			return api.Profile{}, fmt.Errorf("presigning profile avatar: %w", err)
		}
		out.Avatar = &api.ProfileImage{Url: url}
	}

	return out, nil
}

// ProfileToRepo maps a generated ProfileInput (already schema-validated by
// the OpenAPI request validator) to a repository.Profile. It does not set
// StytchUserID (comes from the caller, not the body), AvatarPath (managed
// only via the image endpoints - the update path carries it forward from
// the stored item), or the derived GSI attributes (the repository sets
// those).
func ProfileToRepo(in api.ProfileInput) repository.Profile {
	p := repository.Profile{
		Username:        in.Username,
		DiscordUsername: in.DiscordUsername,
		Bio:             in.Bio,
		Links:           profileLinksToRepo(in.Links),
	}
	if in.Discoverable != nil {
		p.Discoverable = *in.Discoverable
	}

	return p
}

func profileLinksToAPI(links []repository.ProfileLink) *[]api.ProfileLink {
	if len(links) == 0 {
		return nil
	}

	out := make([]api.ProfileLink, len(links))
	for i, l := range links {
		out[i] = api.ProfileLink{Name: l.Name, Url: l.URL}
	}

	return &out
}

func profileLinksToRepo(links *[]api.ProfileLink) []repository.ProfileLink {
	if links == nil || len(*links) == 0 {
		return nil
	}

	out := make([]repository.ProfileLink, len(*links))
	for i, l := range *links {
		out[i] = repository.ProfileLink{Name: l.Name, URL: l.Url}
	}

	return out
}
