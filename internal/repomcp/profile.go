package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileToMCP maps a repository.Profile to its MCP tool shape. The avatar
// is reported as a bool (has_avatar) - MCP never serves image URLs, matching
// SwitchToMCP's HasImage. The owner's IdP subject is never included.
func ProfileToMCP(p repository.Profile) schema.Profile {
	return schema.Profile{
		Username:        p.Username,
		Discoverable:    p.Discoverable,
		DiscordUsername: p.DiscordUsername,
		Bio:             p.Bio,
		Links:           profileLinksToMCP(p.Links),
		HasAvatar:       p.AvatarPath != nil,
	}
}

// ProfileFromMCP maps a create_profile / update_profile tool input to a
// repository.Profile. It does not set StytchUserID (the caller, from ctx),
// AvatarPath (managed only via the image tools), or the GSI discriminators
// (the repository derives those).
func ProfileFromMCP(in schema.ProfileInput) repository.Profile {
	return repository.Profile{
		Username:        in.Username,
		Discoverable:    in.Discoverable,
		DiscordUsername: in.DiscordUsername,
		Bio:             in.Bio,
		Links:           profileLinksFromMCP(in.Links),
	}
}

func profileLinksFromMCP(links []schema.ProfileLink) []repository.ProfileLink {
	if len(links) == 0 {
		return nil
	}

	out := make([]repository.ProfileLink, len(links))
	for i, l := range links {
		out[i] = repository.ProfileLink{Name: l.Name, URL: l.URL}
	}

	return out
}

func profileLinksToMCP(links []repository.ProfileLink) []schema.ProfileLink {
	if len(links) == 0 {
		return nil
	}

	out := make([]schema.ProfileLink, len(links))
	for i, l := range links {
		out[i] = schema.ProfileLink{Name: l.Name, URL: l.URL}
	}

	return out
}
