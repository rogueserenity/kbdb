package mcp

import (
	"context"
	"errors"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/profileread"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errProfileNotFound = errors.New("profile not found")

var getProfileTool = &mcp.Tool{
	Name:        "get_profile",
	Description: "Returns one user's public profile. identifier is either the user's id or their username. A profile that isn't discoverable is only visible to its owner; pass your own user id to read your own non-discoverable profile.",
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
