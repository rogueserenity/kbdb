package repository

import (
	"context"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

// ProfileLink is one entry in a profile's links list. Both fields are
// required and non-blank; validation lives in internal/profilevalidate.
type ProfileLink struct {
	Name string `dynamodbav:"name" json:"name"`
	URL  string `dynamodbav:"url"  json:"url"`
}

// Profile is a user's public identity - one per user, partitioned by the
// IdP subject (OwnerID), no sort key.
//
// OwnerID is never accepted in a request body, but it is returned on
// every read - as user_id on the single-profile API/MCP response and as
// ProfileSummary.user_id in the directory list - since callers need it to
// address the {userId}-keyed collection routes. The json:"-" here is because
// the wire mapping lives in internal/repoapi / internal/repomcp, not because
// the value is withheld.
type Profile struct {
	OwnerID string `dynamodbav:"user_id" json:"-"`

	Username string `dynamodbav:"username" json:"username"`

	Discoverable bool `dynamodbav:"discoverable" json:"discoverable"`

	DiscordUsername *string       `dynamodbav:"discord_username,omitempty" json:"discord_username,omitempty"`
	Bio             *string       `dynamodbav:"bio,omitempty" json:"bio,omitempty"`
	Links           []ProfileLink `dynamodbav:"links,omitempty" json:"links,omitempty"`

	// AvatarPath is managed only via the image endpoints; a body PUT carries
	// it forward from the stored item rather than touching it.
	AvatarPath *ProfileImageKey `dynamodbav:"avatar_path,omitempty" json:"-"`

	// DiscordUsernameLC is the lowercased DiscordUsername, the sort key of
	// DiscoverableDiscordIndex. Set only when Discoverable && DiscordUsername != nil.
	DiscordUsernameLC *string `dynamodbav:"discord_username_lc,omitempty" json:"-"`

	// DiscoverablePK / DiscordPK are the constant ("1") partition keys of the
	// two sparse directory GSIs, written only when the profile belongs in
	// each index (DiscordPK additionally requires DiscordUsername != nil).
	// Left nil - and omitted - otherwise, which is what keeps non-discoverable
	// profiles out of the indexes without a filter expression.
	DiscoverablePK *string `dynamodbav:"discoverable_pk,omitempty" json:"-"`
	DiscordPK      *string `dynamodbav:"discord_pk,omitempty" json:"-"`
}

// ProfileRepository provides access to profiles. Reads take an explicit
// ownerID (a profile is readable by anyone, subject to the caller's
// discoverable check); writes read the caller from ctx, since a user can
// only write their own profile.
type ProfileRepository interface {
	// Get returns the profile keyed by ownerID, or ErrNotFound. Applies
	// no visibility check - the caller does that against Discoverable.
	Get(ctx context.Context, ownerID string) (*Profile, error)

	// ResolveUsername returns the IdP subject that owns username, or
	// ErrNotFound.
	ResolveUsername(ctx context.Context, username string) (ownerID string, err error)

	// Create writes p as the caller's profile plus the { username -> user_id }
	// claim, atomically. Returns ErrAlreadyExists (this user already has a
	// profile) or ErrUsernameTaken (the username belongs to someone else).
	Create(ctx context.Context, p Profile) (*Profile, error)

	// Update replaces the body-settable fields of the caller's profile
	// (username, discoverable, discord_username, bio, links); AvatarPath
	// carries forward from the stored item. An omitted body field is
	// cleared. Returns ErrNotFound if the caller has no profile, or
	// ErrUsernameTaken if a changed username is claimed by a different user.
	Update(ctx context.Context, p Profile) (*Profile, error)

	// Delete removes the caller's profile and its { username -> user_id }
	// claim, atomically. A missing profile is a no-op success (idempotent).
	// This is a leaf delete: nothing references a Profile, so there is no
	// cascade.
	Delete(ctx context.Context) error

	// ListPublic returns a cursor-paginated page of discoverable profiles.
	// At most one of usernamePrefix / discordPrefix may be set (the caller
	// enforces this), each a begins_with filter on its directory index.
	ListPublic(ctx context.Context, usernamePrefix, discordPrefix string, limit int, cursor string) ([]Profile, string, error)

	// SetAvatarPath sets the caller's profile's AvatarPath to key. Returns
	// ErrNotFound if the caller has no profile.
	SetAvatarPath(ctx context.Context, key ProfileImageKey) error

	// ClearAvatarPath clears the caller's profile's AvatarPath and returns
	// the key that was cleared, or nil if it was already unset (idempotent,
	// not an error). Returns ErrNotFound if the caller has no profile.
	ClearAvatarPath(ctx context.Context) (*ProfileImageKey, error)
}

// ProfileImageKey is the object key a profile's avatar is stored under.
type ProfileImageKey string

// NewProfileImageKey builds the caller's avatar object key. ownerID comes
// from ctx so a caller can't address another user's prefix. Fixed key, no
// per-image id - a re-upload overwrites in place.
func NewProfileImageKey(ctx context.Context) (ProfileImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return ProfileImageKey(fmt.Sprintf("profiles/%s/avatar", ownerID)), nil
}

// ProfileImageStore stores a profile's avatar object in a private object
// store, addressed by ProfileImageKey.
type ProfileImageStore interface {
	// PresignGet returns a short-lived presigned GET URL for key.
	PresignGet(ctx context.Context, key ProfileImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked to
	// contentType via the Content-Type header the upload must match.
	PresignPut(ctx context.Context, key ProfileImageKey, contentType string) (url string, err error)

	// Delete removes the object at key. Idempotent, matching S3 DeleteObject.
	Delete(ctx context.Context, key ProfileImageKey) error
}
