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
