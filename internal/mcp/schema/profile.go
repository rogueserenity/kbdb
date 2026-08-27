package schema

// ProfileLink is one entry in a profile's links list.
type ProfileLink struct {
	Name string `json:"name" jsonschema:"display label for the link, e.g. \"Twitch\""`
	URL  string `json:"url" jsonschema:"an https URL; http and other schemes are rejected"`
}

// GetProfileInput is the get_profile tool arguments.
type GetProfileInput struct {
	Identifier string `json:"identifier" jsonschema:"a profile owner's user id or their username"`
}

// GetProfileOutput is the get_profile tool result.
type GetProfileOutput struct {
	Profile Profile `json:"profile" jsonschema:"the requested profile"`
}

// Profile is a user's public identity. HasAvatar reports whether an avatar
// is on file rather than the image itself - images are served to end users
// through REST, not this MCP surface.
type Profile struct {
	Username        string       `json:"username" jsonschema:"the profile's unique username"`
	Discoverable    bool         `json:"discoverable" jsonschema:"whether the profile is listed in the public directory"`
	DiscordUsername *string      `json:"discord_username,omitempty" jsonschema:"the owner's Discord username, if given"`
	Bio             *string      `json:"bio,omitempty" jsonschema:"free-form bio, if given"`
	Links           []ProfileLink `json:"links,omitempty" jsonschema:"the profile's links (name/url pairs)"`
	HasAvatar       bool         `json:"has_avatar" jsonschema:"whether this profile has an avatar on file"`
}
