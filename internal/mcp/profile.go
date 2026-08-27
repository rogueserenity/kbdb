package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/profileread"
	"github.com/rogueserenity/kbdb/internal/profilevalidate"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errProfileNotFound = errors.New("profile not found")

var errProfileAlreadyExists = errors.New("you already have a profile")

var getProfileTool = &mcp.Tool{
	Name:        "get_profile",
	Description: "Returns one user's public profile. identifier is either the user's id or their username. A profile that isn't discoverable is only visible to its owner; pass your own user id to read your own non-discoverable profile.",
}

var createProfileTool = &mcp.Tool{
	Name:        "create_profile",
	Description: "Creates your own public profile. You may have only one profile - creating a second fails. username must be 3-20 chars of lowercase letters, digits, hyphen, or underscore, must not start with \"user-\", and must be unique across all users. Omitting an optional field (discord_username, bio, links) leaves it unset.",
}

var updateProfileTool = &mcp.Tool{
	Name:        "update_profile",
	Description: "Replaces your own public profile. This is a full replace: every field is overwritten, so omitting bio or links clears them - send the complete profile, not just the fields you want to change. The avatar is not part of this call and is left untouched. Fails if you have no profile yet, or if the requested username is already taken by another user.",
}

func handleGetProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.GetProfileInput, schema.GetProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetProfileInput) (*mcp.CallToolResult, schema.GetProfileOutput, error) {
		if strings.TrimSpace(in.Identifier) == "" {
			return nil, schema.GetProfileOutput{}, errors.New("identifier must not be blank")
		}

		p, ok, err := profileread.Resolve(ctx, repo, in.Identifier)
		if err != nil {
			log.FromContext(ctx).Error("getting profile", log.Error, err, log.ProfileID, in.Identifier)
			return nil, schema.GetProfileOutput{}, errors.New("failed to get profile")
		}
		if !ok {
			return nil, schema.GetProfileOutput{}, errProfileNotFound
		}

		return nil, schema.GetProfileOutput{Profile: repomcp.ProfileToMCP(*p)}, nil
	}
}

func handleCreateProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.CreateProfileInput, schema.CreateProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateProfileInput) (*mcp.CallToolResult, schema.CreateProfileOutput, error) {
		p, err := validatedProfile(in.ProfileInput)
		if err != nil {
			return nil, schema.CreateProfileOutput{}, err
		}

		created, err := repo.Create(ctx, p)
		switch {
		case errors.Is(err, repository.ErrAlreadyExists):
			return nil, schema.CreateProfileOutput{}, errProfileAlreadyExists
		case errors.Is(err, repository.ErrUsernameTaken):
			return nil, schema.CreateProfileOutput{}, fmt.Errorf("username %q is already taken", p.Username)
		case err != nil:
			log.FromContext(ctx).Error("creating profile", log.Error, err)
			return nil, schema.CreateProfileOutput{}, errors.New("failed to create profile")
		}

		return nil, schema.CreateProfileOutput{Profile: repomcp.ProfileToMCP(*created)}, nil
	}
}

func handleUpdateProfile(repo repository.ProfileRepository) mcp.ToolHandlerFor[schema.UpdateProfileInput, schema.UpdateProfileOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateProfileInput) (*mcp.CallToolResult, schema.UpdateProfileOutput, error) {
		p, err := validatedProfile(in.ProfileInput)
		if err != nil {
			return nil, schema.UpdateProfileOutput{}, err
		}

		updated, err := repo.Update(ctx, p)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, schema.UpdateProfileOutput{}, errProfileNotFound
		case errors.Is(err, repository.ErrUsernameTaken):
			return nil, schema.UpdateProfileOutput{}, fmt.Errorf("username %q is already taken", p.Username)
		case err != nil:
			log.FromContext(ctx).Error("updating profile", log.Error, err)
			return nil, schema.UpdateProfileOutput{}, errors.New("failed to update profile")
		}

		return nil, schema.UpdateProfileOutput{Profile: repomcp.ProfileToMCP(*updated)}, nil
	}
}

// validatedProfile runs the full field rule set (the SDK enforces none of
// it) and maps the input to a repository.Profile, joining every violation
// into one error like validatedSwitch.
func validatedProfile(in schema.ProfileInput) (repository.Profile, error) {
	p := repomcp.ProfileFromMCP(in)

	fieldErrs := profilevalidate.Validate(p)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = fe.Name + ": " + fe.Reason
		}

		return repository.Profile{}, errors.New(strings.Join(reasons, "; "))
	}

	return p, nil
}
