package repository

import (
	"context"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

// ProfileLink is one entry in a profile's links list: a display name paired
// with an https URL (e.g. a Twitch channel, a Discord invite, an Instagram
// profile). Both fields are required and non-blank; the url is always https
// - validation lives in internal/handlers and internal/mcp, not here.
type ProfileLink struct {
	Name string `dynamodbav:"name" json:"name"`
	URL  string `dynamodbav:"url"  json:"url"`
}

// Profile is a user's public identity - one per user, partitioned by the
// IdP-issued subject claim (StytchUserID). Unlike every other entity table
// there is no sort key: a user has exactly one profile.
//
// Only Username is required. discoverable governs directory visibility:
// when false, only the owner may read the profile and to everyone else it
// behaves as if it doesn't exist.
//
// StytchUserID is never sent in a request body and never appears in the
// single-profile API/MCP response; it surfaces only as user_id in the
// directory list, and only for discoverable profiles.
type Profile struct {
	StytchUserID string `dynamodbav:"user_id" json:"-"`

	Username string `dynamodbav:"username" json:"username"`

	Discoverable bool `dynamodbav:"discoverable" json:"discoverable"`

	DiscordUsername *string `dynamodbav:"discord_username,omitempty" json:"discord_username,omitempty"`
	Bio             *string `dynamodbav:"bio,omitempty" json:"bio,omitempty"`
	Links           []ProfileLink `dynamodbav:"links,omitempty" json:"links,omitempty"`

	// AvatarPath is the object key of the profile's avatar in a
	// ProfileImageStore, or nil. Never sent or received in an API body -
	// the avatar is managed only via the dedicated image endpoints, and a
	// body PUT must carry this forward from the stored item rather than
	// wiping it (see the Update contract).
	AvatarPath *ProfileImageKey `dynamodbav:"avatar_path,omitempty" json:"-"`

	// DiscordUsernameLC is the lowercased DiscordUsername, used as the sort
	// key of the discord-search GSI (discord_username is free-text
	// mixed-case, unlike username which is lowercase by charset). Set only
	// when Discoverable is true and DiscordUsername is non-nil.
	DiscordUsernameLC *string `dynamodbav:"discord_username_lc,omitempty" json:"-"`

	// DiscoverablePK / DiscordPK are the constant ("1") partition keys of
	// the two sparse directory GSIs. Written only when Discoverable is true
	// (DiscordPK additionally requires DiscordUsername != nil); left nil -
	// and so omitted from the item - otherwise, which is what keeps
	// non-discoverable profiles out of the indexes without a filter
	// expression.
	DiscoverablePK *string `dynamodbav:"discoverable_pk,omitempty" json:"-"`
	DiscordPK      *string `dynamodbav:"discord_pk,omitempty" json:"-"`

	// Version is a repository-internal CAS guard against lost updates on
	// concurrent whole-item rewrites (avatar mutations vs. a body PUT), not
	// exposed via the API. Same mechanism as Switch.Version.
	Version int `dynamodbav:"version" json:"-"`
}

// ProfileRepository provides access to profiles. Reads take an explicit
// stytchUserID since a profile can be read by anyone (subject to the
// discoverable check, done by the caller). Writes - added in later issues -
// read the caller from ctx (internal/ctx.UserID) instead, since a user can
// only ever write their own profile.
type ProfileRepository interface {
	// Get returns the profile whose partition key is stytchUserID, or
	// ErrNotFound. Get is keyed by IdP subject only; resolving a username
	// to a subject is ResolveUsername's job, not Get's. Get applies no
	// visibility check - the caller (a handler) does that against the
	// returned Discoverable field.
	Get(ctx context.Context, stytchUserID string) (*Profile, error)

	// ResolveUsername returns the IdP subject that owns username, or
	// ErrNotFound. username is already the storage key (the username
	// charset is lowercase-only, so there is no case-folding to do here).
	// Used by GET /v1/profile/{identifier} and get_profile when the
	// identifier is a username rather than a subject.
	ResolveUsername(ctx context.Context, username string) (stytchUserID string, err error)

	// Create writes p as the caller's profile. The caller is read from ctx
	// (internal/ctx.UserID), not p.StytchUserID - a user can only ever
	// create their own profile. The profile item and the { username ->
	// user_id } claim item are written atomically; on conflict Create
	// returns ErrAlreadyExists (this user already has a profile) or
	// ErrUsernameTaken (the username belongs to someone else). The sparse
	// directory-GSI discriminators are derived from p and set here, not by
	// the caller.
	Create(ctx context.Context, p Profile) (*Profile, error)
}

// ProfileImageKey is the object key a profile's avatar is stored under in a
// ProfileImageStore.
type ProfileImageKey string

// NewProfileImageKey builds the deterministic object key for the caller's
// avatar. ownerID comes from ctx, not a parameter, so a caller can't build
// a key addressing anyone else's prefix. Fixed, no per-image id or
// extension - a re-upload overwrites the same object, so there's no orphan
// accumulation.
func NewProfileImageKey(ctx context.Context) (ProfileImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return ProfileImageKey(fmt.Sprintf("profiles/%s/avatar", ownerID)), nil
}

// ProfileImageStore stores a profile's avatar object in a private object
// store, addressed by ProfileImageKey. Never called with the caller-facing
// presigned URL - that's minted fresh per request, never persisted.
type ProfileImageStore interface {
	// PresignGet returns a short-lived presigned GET URL for key.
	PresignGet(ctx context.Context, key ProfileImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked to
	// contentType via the Content-Type header the upload must match.
	PresignPut(ctx context.Context, key ProfileImageKey, contentType string) (url string, err error)

	// Delete removes the object at key. Idempotent: a nonexistent key is
	// not an error, matching S3's own DeleteObject semantics.
	Delete(ctx context.Context, key ProfileImageKey) error
}
