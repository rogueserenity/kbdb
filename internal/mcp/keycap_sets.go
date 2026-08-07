package mcp

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errKeycapSetNotFound = errors.New("keycap set not found")

var errKeycapKitNotFound = errors.New("keycap kit not found")

var errKeycapKitHasNoImage = errors.New("keycap kit has no image")

var listKeycapSetsTool = &mcp.Tool{
	Name:        "list_keycap_sets",
	Description: "Lists keycap sets in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_keycap_set for a single set's full details, including its kits. Omit user_id to list your own keycap sets.",
}

var getKeycapSetTool = &mcp.Tool{
	Name:        "get_keycap_set",
	Description: "Returns the full details of one keycap set, including its kits. Each kit reports has_image rather than a URL - call get_keycap_kit_image_url to fetch one. Omit user_id to read from your own collection.",
}

var getKeycapKitImageURLTool = &mcp.Tool{
	Name:        "get_keycap_kit_image_url",
	Description: "Mints a short-lived URL to fetch a kit's image. Call this only when you need the image itself; get_keycap_set already reports whether one exists via has_image.",
}

func handleListKeycapSets(repo repository.KeycapSetRepository) mcp.ToolHandlerFor[schema.ListKeycapSetsInput, schema.ListKeycapSetsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeycapSetsInput) (*mcp.CallToolResult, schema.ListKeycapSetsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListKeycapSetsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		sets, nextCursor, err := repo.List(ctx, ownerID, visibilities, clampListLimit(in.Limit), in.Cursor)
		if err != nil {
			log.FromContext(ctx).Error("listing keycap sets", log.Error, err)
			return nil, schema.ListKeycapSetsOutput{}, errors.New("failed to list keycap sets")
		}

		items := make([]schema.KeycapSetSummary, len(sets))
		for i, ks := range sets {
			items[i] = repomcp.KeycapSetToMCPSummary(ks)
		}

		return nil, schema.ListKeycapSetsOutput{KeycapSets: items, NextCursor: nextCursor}, nil
	}
}

func handleGetKeycapSet(repo repository.KeycapSetRepository) mcp.ToolHandlerFor[schema.GetKeycapSetInput, schema.GetKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeycapSetInput) (*mcp.CallToolResult, schema.GetKeycapSetOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.GetKeycapSetOutput{}, errors.New("keycap_set_id must not be blank")
		}

		ks, err := ownedReadable(ctx, repo.Get, func(ks repository.KeycapSet) repository.Visibility { return ks.Visibility },
			"keycap set", errKeycapSetNotFound, log.KeycapSetID, in.UserID, in.KeycapSetID)
		if err != nil {
			return nil, schema.GetKeycapSetOutput{}, err
		}

		return nil, schema.GetKeycapSetOutput{KeycapSet: repomcp.KeycapSetToMCP(*ks)}, nil
	}
}

func handleGetKeycapKitImageURL(
	repo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.GetKeycapKitImageURLInput, schema.GetKeycapKitImageURLOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeycapKitImageURLInput) (*mcp.CallToolResult, schema.GetKeycapKitImageURLOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.GetKeycapKitImageURLOutput{}, errors.New("keycap_set_id must not be blank")
		}
		if strings.TrimSpace(in.KitID) == "" {
			return nil, schema.GetKeycapKitImageURLOutput{}, errors.New("kit_id must not be blank")
		}

		ks, err := ownedReadable(ctx, repo.Get, func(ks repository.KeycapSet) repository.Visibility { return ks.Visibility },
			"keycap set", errKeycapSetNotFound, log.KeycapSetID, in.UserID, in.KeycapSetID)
		if err != nil {
			return nil, schema.GetKeycapKitImageURLOutput{}, err
		}

		idx := slices.IndexFunc(ks.Kits, func(k repository.KeycapKit) bool { return k.KitID == in.KitID })
		if idx == -1 {
			return nil, schema.GetKeycapKitImageURLOutput{}, errKeycapKitNotFound
		}

		kit := ks.Kits[idx]
		if kit.ImagePath == nil {
			return nil, schema.GetKeycapKitImageURLOutput{}, errKeycapKitHasNoImage
		}

		url, err := images.PresignGet(ctx, *kit.ImagePath)
		if err != nil {
			log.FromContext(ctx).Error("presigning keycap kit image", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.GetKeycapKitImageURLOutput{}, errors.New("failed to presign keycap kit image")
		}

		return nil, schema.GetKeycapKitImageURLOutput{URL: url}, nil
	}
}
