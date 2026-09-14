package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// ProfileToMCP maps a repository.Profile to its MCP tool shape: avatar as a
// bool.
func ProfileToMCP(p repository.Profile) schema.Profile {
	prefs := profilePreferencesToMCP(p.Preferences)
	return schema.Profile{
		Username:        p.Username,
		UserID:          p.OwnerID,
		Discoverable:    p.Discoverable,
		DiscordUsername: p.DiscordUsername,
		Bio:             p.Bio,
		Links:           profileLinksToMCP(p.Links),
		HasAvatar:       p.AvatarPath != nil,
		Preferences:     prefs,
	}
}

// ProfileToMCPSummary maps a repository.Profile to a list_profiles row -
// no bio or links, avatar as a bool.
func ProfileToMCPSummary(p repository.Profile) schema.ProfileSummary {
	return schema.ProfileSummary{
		Username:        p.Username,
		UserID:          p.OwnerID,
		DiscordUsername: p.DiscordUsername,
		HasAvatar:       p.AvatarPath != nil,
	}
}

// ProfileFromMCP maps a create_profile / update_profile tool input to a
// repository.Profile. OwnerID, AvatarPath, and the GSI discriminators
// are set downstream, not here.
func ProfileFromMCP(in schema.ProfileInput) repository.Profile {
	prefs := repository.DefaultProfilePreferences()
	if in.Preferences != nil {
		prefs = profilePreferencesFromMCP(*in.Preferences)
	}

	return repository.Profile{
		Username:        in.Username,
		Discoverable:    in.Discoverable,
		DiscordUsername: in.DiscordUsername,
		Bio:             in.Bio,
		Links:           profileLinksFromMCP(in.Links),
		Preferences:     prefs,
	}
}

func profilePreferencesToMCP(p repository.ProfilePreferences) schema.ProfilePreferences {
	return schema.ProfilePreferences{
		Currency:          p.Currency,
		ShowPriceToMe:     p.ShowPriceToMe,
		ShowPriceToOthers: p.ShowPriceToOthers,
	}
}

func profilePreferencesFromMCP(in schema.ProfilePreferences) repository.ProfilePreferences {
	return repository.ProfilePreferences{
		Currency:          in.Currency,
		ShowPriceToMe:     in.ShowPriceToMe,
		ShowPriceToOthers: in.ShowPriceToOthers,
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
