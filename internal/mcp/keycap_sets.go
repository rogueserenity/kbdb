package mcp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rogueserenity/kbdb/internal/authz"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errKeycapSetNotFound = errors.New("keycap set not found")

var errKeycapSetAlreadyExists = errors.New("keycap set already exists")

var errKeycapKitNotFound = errors.New("keycap kit not found")

var errKeycapKitHasNoImage = errors.New("keycap kit has no image")

var errKeycapSetMutationConflict = errors.New("the keycap set is being modified concurrently, please retry")

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

var createKeycapSetTool = &mcp.Tool{
	Name:        "create_keycap_set",
	Description: "Adds a keycap set to your own collection. profile and material must be approved lookup values - call list_lookups and get_lookup to see them. Kits aren't set here - add them afterward with create_keycap_kit.",
}

var updateKeycapSetTool = &mcp.Tool{
	Name:        "update_keycap_set",
	Description: "Replaces a keycap set in your own collection. Every field is replaced, so omitting an optional field clears it; send the full keycap set, not just the fields you want to change. Kits are preserved untouched - manage them with the kit tools instead.",
}

var deleteKeycapSetTool = &mcp.Tool{
	Name:        "delete_keycap_set",
	Description: "Removes a keycap set from your own collection, along with any kit images. Idempotent: deleting a keycap set that isn't there succeeds.",
}

var createKeycapKitTool = &mcp.Tool{
	Name:        "create_keycap_kit",
	Description: "Adds a kit to a keycap set in your own collection. purchase.vendor and purchase.order_status must be approved lookup values - call list_lookups and get_lookup to see them. To set the kit's image, call set_keycap_kit_image afterward.",
}

var updateKeycapKitTool = &mcp.Tool{
	Name:        "update_keycap_kit",
	Description: "Replaces a kit on a keycap set in your own collection. Every field is replaced, so omitting an optional field clears it; send the full kit, not just the fields you want to change. The kit's image, if any, is preserved - manage it with the image tools instead.",
}

var deleteKeycapKitTool = &mcp.Tool{
	Name:        "delete_keycap_kit",
	Description: "Removes a kit from a keycap set in your own collection, along with its image if it has one. Idempotent: deleting a kit that isn't there succeeds.",
}

var setKeycapKitImageTool = &mcp.Tool{
	Name:        "set_keycap_kit_image",
	Description: "Mints a presigned URL to upload a kit's image to. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header.",
}

var deleteKeycapKitImageTool = &mcp.Tool{
	Name:        "delete_keycap_kit_image",
	Description: "Removes a kit's image. Idempotent: deleting a kit's image when it doesn't have one succeeds.",
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

func handleCreateKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.CreateKeycapSetInput, schema.CreateKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeycapSetInput) (*mcp.CallToolResult, schema.CreateKeycapSetOutput, error) {
		ks, err := validatedKeycapSet(ctx, in.KeycapSetInput)
		if err != nil {
			return nil, schema.CreateKeycapSetOutput{}, err
		}

		ks.ID = uuid.NewString()

		created, err := keycapSetRepo.Create(ctx, ks)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateKeycapSetOutput{}, errKeycapSetAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating keycap set", log.KeycapSetID, ks.ID, log.Error, err)
			return nil, schema.CreateKeycapSetOutput{}, errors.New("failed to create keycap set")
		}

		return nil, schema.CreateKeycapSetOutput{KeycapSet: repomcp.KeycapSetToMCP(*created)}, nil
	}
}

func handleUpdateKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.UpdateKeycapSetInput, schema.UpdateKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeycapSetInput) (*mcp.CallToolResult, schema.UpdateKeycapSetOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.UpdateKeycapSetOutput{}, errors.New("keycap_set_id must not be blank")
		}

		ks, err := validatedKeycapSet(ctx, in.KeycapSetInput)
		if err != nil {
			return nil, schema.UpdateKeycapSetOutput{}, err
		}

		ks.ID = in.KeycapSetID

		updated, err := keycapSetRepo.Update(ctx, ks)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.UpdateKeycapSetOutput{}, errKeycapSetNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			// Warn, not Error: expected contention under retry, not a bug -
			// still worth a trace if one set sees this repeatedly.
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, ks.ID)
			return nil, schema.UpdateKeycapSetOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("updating keycap set", log.KeycapSetID, ks.ID, log.Error, err)
			return nil, schema.UpdateKeycapSetOutput{}, errors.New("failed to update keycap set")
		}

		return nil, schema.UpdateKeycapSetOutput{KeycapSet: repomcp.KeycapSetToMCP(*updated)}, nil
	}
}

func handleDeleteKeycapSet(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapSetInput, schema.DeleteKeycapSetOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapSetInput) (*mcp.CallToolResult, schema.DeleteKeycapSetOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.DeleteKeycapSetOutput{}, errors.New("keycap_set_id must not be blank")
		}

		imageKeys, err := keycapSetRepo.Delete(ctx, in.KeycapSetID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			log.FromContext(ctx).Error("deleting keycap set", log.KeycapSetID, in.KeycapSetID, log.Error, err)
			return nil, schema.DeleteKeycapSetOutput{}, errors.New("failed to delete keycap set")
		}

		images.BestEffortDelete(ctx, imageKeys)

		return nil, schema.DeleteKeycapSetOutput{}, nil
	}
}

// validatedKeycapSet checks in code what api/openapi.yaml declares for
// REST: the SDK infers tool schemas from Go types alone, so there is no
// per-field constraint to attach.
func validatedKeycapSet(
	ctx context.Context,
	in schema.KeycapSetInput,
) (repository.KeycapSet, error) {
	if strings.TrimSpace(in.Brand) == "" {
		return repository.KeycapSet{}, errors.New("brand must not be blank")
	}
	if strings.TrimSpace(in.Name) == "" {
		return repository.KeycapSet{}, errors.New("name must not be blank")
	}

	ks := repomcp.KeycapSetFromMCP(in)

	if !ks.Visibility.Valid() {
		return repository.KeycapSet{}, fmt.Errorf(
			"visibility %q must be one of: public, authenticated, private", in.Visibility)
	}

	fieldErrs := lookup.ValidateKeycapSet(ctx, ks)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
		}

		return repository.KeycapSet{}, errors.New(strings.Join(reasons, "; "))
	}

	return ks, nil
}

func handleCreateKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.CreateKeycapKitInput, schema.CreateKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeycapKitInput) (*mcp.CallToolResult, schema.CreateKeycapKitOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.CreateKeycapKitOutput{}, errors.New("keycap_set_id must not be blank")
		}

		kit, err := validatedKeycapKit(ctx, in.KeycapKitInput)
		if err != nil {
			return nil, schema.CreateKeycapKitOutput{}, err
		}

		kit.KitID = uuid.NewString()

		created, err := keycapSetRepo.AddKit(ctx, in.KeycapSetID, kit)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.CreateKeycapKitOutput{}, errKeycapSetNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, kit.KitID)
			return nil, schema.CreateKeycapKitOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("adding keycap kit", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, kit.KitID, log.Error, err)
			return nil, schema.CreateKeycapKitOutput{}, errors.New("failed to add kit")
		}

		return nil, schema.CreateKeycapKitOutput{KeycapKit: repomcp.KeycapKitToMCP(*created)}, nil
	}
}

func handleUpdateKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
) mcp.ToolHandlerFor[schema.UpdateKeycapKitInput, schema.UpdateKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeycapKitInput) (*mcp.CallToolResult, schema.UpdateKeycapKitOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.UpdateKeycapKitOutput{}, errors.New("keycap_set_id must not be blank")
		}
		if strings.TrimSpace(in.KitID) == "" {
			return nil, schema.UpdateKeycapKitOutput{}, errors.New("kit_id must not be blank")
		}

		kit, err := validatedKeycapKit(ctx, in.KeycapKitInput)
		if err != nil {
			return nil, schema.UpdateKeycapKitOutput{}, err
		}

		kit.KitID = in.KitID

		updated, err := keycapSetRepo.UpdateKit(ctx, in.KeycapSetID, kit)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.UpdateKeycapKitOutput{}, errKeycapKitNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID)
			return nil, schema.UpdateKeycapKitOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("updating keycap kit", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.UpdateKeycapKitOutput{}, errors.New("failed to update kit")
		}

		return nil, schema.UpdateKeycapKitOutput{KeycapKit: repomcp.KeycapKitToMCP(*updated)}, nil
	}
}

func handleDeleteKeycapKit(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapKitInput, schema.DeleteKeycapKitOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapKitInput) (*mcp.CallToolResult, schema.DeleteKeycapKitOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.DeleteKeycapKitOutput{}, errors.New("keycap_set_id must not be blank")
		}
		if strings.TrimSpace(in.KitID) == "" {
			return nil, schema.DeleteKeycapKitOutput{}, errors.New("kit_id must not be blank")
		}

		cleared, err := keycapSetRepo.DeleteKit(ctx, in.KeycapSetID, in.KitID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.DeleteKeycapKitOutput{}, errKeycapSetNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID)
			return nil, schema.DeleteKeycapKitOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting keycap kit", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.DeleteKeycapKitOutput{}, errors.New("failed to delete kit")
		}

		if cleared != nil {
			if err := images.Delete(ctx, *cleared); err != nil {
				log.FromContext(ctx).Warn("deleting keycap kit image object after kit delete", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.KeycapKitImage, *cleared, log.Error, err)
			}
		}

		return nil, schema.DeleteKeycapKitOutput{}, nil
	}
}

func handleSetKeycapKitImage(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.SetKeycapKitImageInput, schema.SetKeycapKitImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.SetKeycapKitImageInput) (*mcp.CallToolResult, schema.SetKeycapKitImageOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("keycap_set_id must not be blank")
		}
		if strings.TrimSpace(in.KitID) == "" {
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("kit_id must not be blank")
		}

		if fieldErr := lookup.ValidateImageContentType(ctx, in.ContentType); fieldErr != nil {
			return nil, schema.SetKeycapKitImageOutput{}, fmt.Errorf("content_type: %q is not an approved %s value", in.ContentType, lookup.CategoryImageContentType)
		}

		key, err := repository.NewKeycapKitImageKey(ctx, in.KeycapSetID, in.KitID)
		if err != nil {
			log.FromContext(ctx).Error("building keycap kit image key", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
		}

		uploadURL, err := images.PresignPut(ctx, key, in.ContentType)
		if err != nil {
			log.FromContext(ctx).Error("presigning keycap kit image upload", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
		}

		_, err = keycapSetRepo.SetKitImagePath(ctx, in.KeycapSetID, in.KitID, key)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.SetKeycapKitImageOutput{}, errKeycapKitNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID)
			return nil, schema.SetKeycapKitImageOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("setting keycap kit image path", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.SetKeycapKitImageOutput{}, errors.New("failed to set kit image")
		}

		return nil, schema.SetKeycapKitImageOutput{UploadURL: uploadURL}, nil
	}
}

func handleDeleteKeycapKitImage(
	keycapSetRepo repository.KeycapSetRepository,
	images repository.KeycapKitImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeycapKitImageInput, schema.DeleteKeycapKitImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeycapKitImageInput) (*mcp.CallToolResult, schema.DeleteKeycapKitImageOutput, error) {
		if strings.TrimSpace(in.KeycapSetID) == "" {
			return nil, schema.DeleteKeycapKitImageOutput{}, errors.New("keycap_set_id must not be blank")
		}
		if strings.TrimSpace(in.KitID) == "" {
			return nil, schema.DeleteKeycapKitImageOutput{}, errors.New("kit_id must not be blank")
		}

		cleared, err := keycapSetRepo.ClearKitImagePath(ctx, in.KeycapSetID, in.KitID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.DeleteKeycapKitImageOutput{}, errKeycapKitNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keycap set mutation conflict", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID)
			return nil, schema.DeleteKeycapKitImageOutput{}, errKeycapSetMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("clearing keycap kit image path", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
			return nil, schema.DeleteKeycapKitImageOutput{}, errors.New("failed to delete kit image")
		}

		if cleared != nil {
			if err := images.Delete(ctx, *cleared); err != nil {
				log.FromContext(ctx).Error("deleting keycap kit image object", log.KeycapSetID, in.KeycapSetID, log.KeycapKitID, in.KitID, log.Error, err)
				return nil, schema.DeleteKeycapKitImageOutput{}, errors.New("failed to delete kit image")
			}
		}

		return nil, schema.DeleteKeycapKitImageOutput{}, nil
	}
}

// validatedKeycapKit checks in code what api/openapi.yaml declares for
// REST: the SDK infers tool schemas from Go types alone, so there is no
// per-field constraint to attach. Unlike validatedKeycapSet, there's no
// visibility to check - a kit has no independent visibility, only the
// parent set's.
func validatedKeycapKit(
	ctx context.Context,
	in schema.KeycapKitInput,
) (repository.KeycapKit, error) {
	if strings.TrimSpace(in.Name) == "" {
		return repository.KeycapKit{}, errors.New("name must not be blank")
	}

	if in.Purchase != nil {
		if err := validatePurchaseDates(in.Purchase.OrderDate, in.Purchase.DeliveryDate); err != nil {
			return repository.KeycapKit{}, err
		}
	}

	kit := repomcp.KeycapKitFromMCP(in)

	fieldErrs := lookup.ValidateKeycapKit(ctx, kit)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
		}

		return repository.KeycapKit{}, errors.New(strings.Join(reasons, "; "))
	}

	return kit, nil
}
