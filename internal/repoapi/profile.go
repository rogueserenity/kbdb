package repoapi

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileToAPI maps a repository.Profile to its wire shape, presigning the
// avatar if set. Errors only if presigning fails.
func ProfileToAPI(ctx context.Context, p repository.Profile, images repository.ProfileImageStore) (api.Profile, error) {
	out := api.Profile{
		Username:        p.Username,
		UserId:          &p.OwnerID,
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

// ProfileToAPISummary maps a repository.Profile to a directory row -
// no bio or links, avatar presigned if set.
func ProfileToAPISummary(ctx context.Context, p repository.Profile, images repository.ProfileImageStore) (api.ProfileSummary, error) {
	summary := api.ProfileSummary{
		Username:        &p.Username,
		UserId:          &p.OwnerID,
		DiscordUsername: p.DiscordUsername,
	}

	if p.AvatarPath != nil {
		url, err := images.PresignGet(ctx, *p.AvatarPath)
		if err != nil {
			return api.ProfileSummary{}, fmt.Errorf("presigning profile avatar: %w", err)
		}
		summary.Avatar = &api.ProfileImage{Url: url}
	}

	return summary, nil
}

// ProfileToRepo maps a ProfileInput to a repository.Profile. OwnerID,
// AvatarPath, and the GSI discriminators are set downstream, not here.
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
