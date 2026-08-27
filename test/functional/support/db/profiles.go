package db

import (
	"context"
	"errors"
	"strings"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

// SeedProfileOptions is the fixture shape for SeedProfile. Only Username is
// required; everything else is optional. When Discoverable is true, the
// sparse-GSI discriminators are set the way the app would set them.
type SeedProfileOptions struct {
	Username        string
	Discoverable    bool
	DiscordUsername string
	Bio             string
	Links           []map[string]string
	AvatarPath      string
}

// SeedProfile PutItems a profile directly into DynamoDB and writes the
// matching { username -> user_id } claim item, bypassing the API - for
// specs that need a profile in place before exercising the read routes,
// which is every profile spec until the create route exists.
//
// version is set to 0, matching what Create will set it to.
func SeedProfile(ctx context.Context, ownerID string, opts SeedProfileOptions) error {
	profileTable := NewDynamoTable(ctx, support.ProfileTableName())

	item := map[string]any{
		"user_id":      ownerID,
		"username":     opts.Username,
		"discoverable": opts.Discoverable,
		"version":      0,
	}
	if opts.DiscordUsername != "" {
		item["discord_username"] = opts.DiscordUsername
	}
	if opts.Bio != "" {
		item["bio"] = opts.Bio
	}
	if len(opts.Links) > 0 {
		item["links"] = opts.Links
	}
	if opts.AvatarPath != "" {
		item["avatar_path"] = opts.AvatarPath
	}
	if opts.Discoverable {
		item["discoverable_pk"] = "1"
		if opts.DiscordUsername != "" {
			item["discord_pk"] = "1"
			item["discord_username_lc"] = strings.ToLower(opts.DiscordUsername)
		}
	}

	if err := profileTable.PutItem(ctx, item); err != nil {
		return err
	}

	usernameTable := NewDynamoTable(ctx, support.ProfileUsernameTableName())
	return usernameTable.PutItem(ctx, map[string]any{
		"username": opts.Username,
		"user_id":  ownerID,
	})
}

// DeleteProfile removes a profile fixture and its username claim item.
func DeleteProfile(ctx context.Context, ownerID, username string) error {
	profileErr := NewDynamoTable(ctx, support.ProfileTableName()).
		DeleteItem(ctx, map[string]string{"user_id": ownerID})
	claimErr := NewDynamoTable(ctx, support.ProfileUsernameTableName()).
		DeleteItem(ctx, map[string]string{"username": username})

	return errors.Join(profileErr, claimErr)
}
