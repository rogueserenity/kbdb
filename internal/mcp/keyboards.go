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
	"github.com/rogueserenity/kbdb/internal/cascadedelete"
	"github.com/rogueserenity/kbdb/internal/log"
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repomcp"
	"github.com/rogueserenity/kbdb/internal/repository"
)

var errKeyboardNotFound = errors.New("keyboard not found")

var errKeyboardAlreadyExists = errors.New("keyboard already exists")

var errKeyboardImageNotFound = errors.New("keyboard image not found")

var errKeyboardMutationConflict = errors.New("the keyboard is being modified concurrently, please retry")

var listKeyboardsTool = &mcp.Tool{
	Name:        "list_keyboards",
	Description: "Lists keyboards in a user's collection, most useful for browsing. Returns an abbreviated shape; call get_keyboard for a single keyboard's full details. Omit user_id to list your own keyboards.",
}

var getKeyboardTool = &mcp.Tool{
	Name:        "get_keyboard",
	Description: "Returns the full details of one keyboard, including its case/plate design, PCB and purchase history. Omit user_id to read from your own collection.",
}

var createKeyboardTool = &mcp.Tool{
	Name:        "create_keyboard",
	Description: "Adds a keyboard to your own collection. size, layout, the design materials, pcb fields and purchase vendor/status must be approved lookup values - call list_lookups and get_lookup to see them. layout must additionally be valid for the chosen size; get_lookup(\"keyboard_layout\") lists the sizes each layout allows.",
}

var updateKeyboardTool = &mcp.Tool{
	Name:        "update_keyboard",
	Description: "Replaces a keyboard in your own collection. Every field is replaced, so omitting an optional field clears it; send the full keyboard, not just the fields you want to change.",
}

var deleteKeyboardTool = &mcp.Tool{
	Name:        "delete_keyboard",
	Description: "Removes a keyboard from your own collection. Idempotent: deleting a keyboard that isn't there succeeds. on_delete controls what happens if a build still references this keyboard: \"block\" (default) fails and lists the blocking build ids; \"cascade\" deletes the keyboard and every referencing build; \"detach\" deletes the keyboard regardless, leaving referencing builds with a dangling keyboard_id.",
}

var getKeyboardImageURLTool = &mcp.Tool{
	Name:        "get_keyboard_image_url",
	Description: "Mints a short-lived URL to fetch one of a keyboard's images. Call this only when you need the image itself; get_keyboard/list_keyboards already report whether any exist via has_images.",
}

var addKeyboardImageTool = &mcp.Tool{
	Name:        "add_keyboard_image",
	Description: "Adds an image to a keyboard in your own collection. Doesn't upload the image itself - PUT the image bytes to the returned upload_url using the same content_type as the Content-Type header.",
}

var deleteKeyboardImageTool = &mcp.Tool{
	Name:        "delete_keyboard_image",
	Description: "Removes an image from a keyboard. Idempotent: deleting an image that isn't there succeeds.",
}

func handleListKeyboards(repo repository.KeyboardRepository) mcp.ToolHandlerFor[schema.ListKeyboardsInput, schema.ListKeyboardsOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.ListKeyboardsInput) (*mcp.CallToolResult, schema.ListKeyboardsOutput, error) {
		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.ListKeyboardsOutput{}, err
		}

		visibilities := authz.ReadableVisibilities(ctx, ownerID)

		keyboards, nextCursor, err := repo.List(ctx, ownerID, visibilities, clampListLimit(in.Limit), in.Cursor)
		if err != nil {
			log.FromContext(ctx).Error("listing keyboards", log.Error, err)
			return nil, schema.ListKeyboardsOutput{}, errors.New("failed to list keyboards")
		}

		items := make([]schema.KeyboardSummary, len(keyboards))
		for i, kb := range keyboards {
			items[i] = repomcp.KeyboardToMCPSummary(kb)
		}

		return nil, schema.ListKeyboardsOutput{Keyboards: items, NextCursor: nextCursor}, nil
	}
}

func handleGetKeyboard(repo repository.KeyboardRepository) mcp.ToolHandlerFor[schema.GetKeyboardInput, schema.GetKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeyboardInput) (*mcp.CallToolResult, schema.GetKeyboardOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.GetKeyboardOutput{}, errors.New("keyboard_id must not be blank")
		}

		ownerID, err := resolveOwnerID(ctx, in.UserID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		kb, err := ownedReadable(ctx, repo.Get, func(k repository.Keyboard) repository.Visibility { return k.Visibility },
			"keyboard", errKeyboardNotFound, log.KeyboardID, in.UserID, in.KeyboardID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		return nil, schema.GetKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*kb, authz.IsOwner(ctx, ownerID))}, nil
	}
}

func handleCreateKeyboard(
	keyboardRepo repository.KeyboardRepository,
) mcp.ToolHandlerFor[schema.CreateKeyboardInput, schema.CreateKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.CreateKeyboardInput) (*mcp.CallToolResult, schema.CreateKeyboardOutput, error) {
		kb, err := validatedKeyboard(ctx, in.KeyboardInput)
		if err != nil {
			return nil, schema.CreateKeyboardOutput{}, err
		}

		kb.ID = uuid.NewString()

		created, err := keyboardRepo.Create(ctx, kb)
		if errors.Is(err, repository.ErrAlreadyExists) {
			return nil, schema.CreateKeyboardOutput{}, errKeyboardAlreadyExists
		}
		if err != nil {
			log.FromContext(ctx).Error("creating keyboard", log.KeyboardID, kb.ID, log.Error, err)
			return nil, schema.CreateKeyboardOutput{}, errors.New("failed to create keyboard")
		}

		// isOwner: true - create always targets the caller's own collection.
		return nil, schema.CreateKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*created, true)}, nil
	}
}

func handleUpdateKeyboard(
	keyboardRepo repository.KeyboardRepository,
) mcp.ToolHandlerFor[schema.UpdateKeyboardInput, schema.UpdateKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.UpdateKeyboardInput) (*mcp.CallToolResult, schema.UpdateKeyboardOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.UpdateKeyboardOutput{}, errors.New("keyboard_id must not be blank")
		}

		kb, err := validatedKeyboard(ctx, in.KeyboardInput)
		if err != nil {
			return nil, schema.UpdateKeyboardOutput{}, err
		}

		kb.ID = in.KeyboardID

		updated, err := keyboardRepo.Update(ctx, kb)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.UpdateKeyboardOutput{}, errKeyboardNotFound
		}
		if err != nil {
			log.FromContext(ctx).Error("updating keyboard", log.KeyboardID, kb.ID, log.Error, err)
			return nil, schema.UpdateKeyboardOutput{}, errors.New("failed to update keyboard")
		}

		// isOwner: true - update always targets the caller's own collection.
		return nil, schema.UpdateKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*updated, true)}, nil
	}
}

func handleDeleteKeyboard(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	buildImages repository.BuildImageStore,
	keyboardImages repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardInput, schema.DeleteKeyboardOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardInput) (*mcp.CallToolResult, schema.DeleteKeyboardOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.DeleteKeyboardOutput{}, errors.New("keyboard_id must not be blank")
		}

		onDelete, ok := cascadedelete.ParseOnDelete(in.OnDelete)
		if !ok {
			return nil, schema.DeleteKeyboardOutput{}, errors.New("on_delete must be one of: block, cascade, detach")
		}

		ownerID, err := resolveOwnerID(ctx, "")
		if err != nil {
			return nil, schema.DeleteKeyboardOutput{}, err
		}

		result, err := cascadedelete.DeleteKeyboard(ctx, keyboardRepo, buildRepo, buildImages, ownerID, in.KeyboardID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteKeyboardOutput{}, fmt.Errorf("keyboard is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting keyboard", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.DeleteKeyboardOutput{}, errors.New("failed to delete keyboard")
		}

		keyboardImages.BestEffortDelete(ctx, result.ImageKeys)

		return nil, schema.DeleteKeyboardOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
	}
}

func handleGetKeyboardImageURL(
	repo repository.KeyboardRepository,
	images repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.GetKeyboardImageURLInput, schema.GetKeyboardImageURLOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.GetKeyboardImageURLInput) (*mcp.CallToolResult, schema.GetKeyboardImageURLOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.GetKeyboardImageURLOutput{}, errors.New("keyboard_id must not be blank")
		}
		if strings.TrimSpace(in.ImageID) == "" {
			return nil, schema.GetKeyboardImageURLOutput{}, errors.New("image_id must not be blank")
		}

		kb, err := ownedReadable(ctx, repo.Get, func(k repository.Keyboard) repository.Visibility { return k.Visibility },
			"keyboard", errKeyboardNotFound, log.KeyboardID, in.UserID, in.KeyboardID)
		if err != nil {
			return nil, schema.GetKeyboardImageURLOutput{}, err
		}

		idx := slices.IndexFunc(kb.Images, func(i repository.KeyboardImage) bool { return i.ImageID == in.ImageID })
		if idx == -1 {
			return nil, schema.GetKeyboardImageURLOutput{}, errKeyboardImageNotFound
		}

		url, err := images.PresignGetKeyboardImage(ctx, kb.Images[idx].Path)
		if err != nil {
			log.FromContext(ctx).Error("presigning keyboard image", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.GetKeyboardImageURLOutput{}, errors.New("failed to presign keyboard image")
		}

		return nil, schema.GetKeyboardImageURLOutput{URL: url}, nil
	}
}

func handleAddKeyboardImage(
	keyboardRepo repository.KeyboardRepository,
	images repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.AddKeyboardImageInput, schema.AddKeyboardImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.AddKeyboardImageInput) (*mcp.CallToolResult, schema.AddKeyboardImageOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.AddKeyboardImageOutput{}, errors.New("keyboard_id must not be blank")
		}

		if fieldErr := lookup.ValidateImageContentType(ctx, in.ContentType); fieldErr != nil {
			return nil, schema.AddKeyboardImageOutput{}, fmt.Errorf("content_type: %q is not an approved %s value", in.ContentType, lookup.CategoryImageContentType)
		}

		imageID := uuid.NewString()

		key, err := repository.NewKeyboardImageKey(ctx, in.KeyboardID, imageID)
		if err != nil {
			log.FromContext(ctx).Error("building keyboard image key", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.AddKeyboardImageOutput{}, errors.New("failed to add keyboard image")
		}

		uploadURL, err := images.PresignPutKeyboardImage(ctx, key, in.ContentType)
		if err != nil {
			log.FromContext(ctx).Error("presigning keyboard image upload", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.AddKeyboardImageOutput{}, errors.New("failed to add keyboard image")
		}

		_, err = keyboardRepo.AddImage(ctx, in.KeyboardID, repository.KeyboardImage{ImageID: imageID, Path: key})
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.AddKeyboardImageOutput{}, errKeyboardNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keyboard mutation conflict", log.KeyboardID, in.KeyboardID)
			return nil, schema.AddKeyboardImageOutput{}, errKeyboardMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("adding keyboard image", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.AddKeyboardImageOutput{}, errors.New("failed to add keyboard image")
		}

		return nil, schema.AddKeyboardImageOutput{ImageID: imageID, UploadURL: uploadURL}, nil
	}
}

func handleDeleteKeyboardImage(
	keyboardRepo repository.KeyboardRepository,
	images repository.KeyboardImageStore,
) mcp.ToolHandlerFor[schema.DeleteKeyboardImageInput, schema.DeleteKeyboardImageOutput] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in schema.DeleteKeyboardImageInput) (*mcp.CallToolResult, schema.DeleteKeyboardImageOutput, error) {
		if strings.TrimSpace(in.KeyboardID) == "" {
			return nil, schema.DeleteKeyboardImageOutput{}, errors.New("keyboard_id must not be blank")
		}
		if strings.TrimSpace(in.ImageID) == "" {
			return nil, schema.DeleteKeyboardImageOutput{}, errors.New("image_id must not be blank")
		}

		removed, err := keyboardRepo.DeleteImage(ctx, in.KeyboardID, in.ImageID)
		if errors.Is(err, repository.ErrNotFound) {
			return nil, schema.DeleteKeyboardImageOutput{}, errKeyboardNotFound
		}
		if errors.Is(err, repository.ErrMutationConflict) {
			log.FromContext(ctx).Warn("keyboard mutation conflict", log.KeyboardID, in.KeyboardID)
			return nil, schema.DeleteKeyboardImageOutput{}, errKeyboardMutationConflict
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting keyboard image", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.DeleteKeyboardImageOutput{}, errors.New("failed to delete keyboard image")
		}

		if removed != nil {
			if err := images.DeleteKeyboardImage(ctx, *removed); err != nil {
				log.FromContext(ctx).Error("deleting keyboard image object", log.KeyboardID, in.KeyboardID, log.Error, err)
				return nil, schema.DeleteKeyboardImageOutput{}, errors.New("failed to delete keyboard image")
			}
		}

		return nil, schema.DeleteKeyboardImageOutput{}, nil
	}
}

func validatedKeyboard(
	ctx context.Context,
	in schema.KeyboardInput,
) (repository.Keyboard, error) {
	if strings.TrimSpace(in.Brand) == "" {
		return repository.Keyboard{}, errors.New("brand must not be blank")
	}
	if strings.TrimSpace(in.Name) == "" {
		return repository.Keyboard{}, errors.New("name must not be blank")
	}

	if in.Purchase != nil {
		if err := validatePurchaseDates(in.Purchase.OrderDate, in.Purchase.DeliveryDate); err != nil {
			return repository.Keyboard{}, err
		}
	}

	kb := repomcp.KeyboardFromMCP(in)

	if !kb.Visibility.Valid() {
		return repository.Keyboard{}, fmt.Errorf(
			"visibility %q must be one of: public, authenticated, private", in.Visibility)
	}

	fieldErrs := lookup.ValidateKeyboard(ctx, kb)
	if len(fieldErrs) > 0 {
		reasons := make([]string, len(fieldErrs))
		for i, fe := range fieldErrs {
			reasons[i] = keyboardFieldErrorReason(fe, in.Size)
		}

		return repository.Keyboard{}, errors.New(strings.Join(reasons, "; "))
	}

	return kb, nil
}

// ValidateKeyboard reports a layout that isn't valid for the chosen size as
// a FieldError on "layout" carrying the *size* category, which the generic
// wording would render as "layout is not an approved keyboard_size value" -
// telling an agent to go fix a field that's already correct. Mirrors
// [github.com/rogueserenity/kbdb/internal/handlers.keyboardFieldErrorToInvalidParam].
func keyboardFieldErrorReason(fe lookup.FieldError, size *string) string {
	if fe.Field == "layout" && fe.Category == lookup.CategoryKeyboardSize && size != nil {
		return fmt.Sprintf("layout: %q is not a valid layout for size %q", fe.Value, *size)
	}

	return fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
}
