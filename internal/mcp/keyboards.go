package mcp

import (
	"context"
	"errors"
	"fmt"
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

		kb, err := ownedReadable(ctx, repo.Get, func(k repository.Keyboard) repository.Visibility { return k.Visibility },
			"keyboard", errKeyboardNotFound, log.KeyboardID, in.UserID, in.KeyboardID)
		if err != nil {
			return nil, schema.GetKeyboardOutput{}, err
		}

		return nil, schema.GetKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*kb)}, nil
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

		return nil, schema.CreateKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*created)}, nil
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

		return nil, schema.UpdateKeyboardOutput{Keyboard: repomcp.KeyboardToMCP(*updated)}, nil
	}
}

func handleDeleteKeyboard(
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
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

		result, err := cascadedelete.DeleteKeyboard(ctx, keyboardRepo, buildRepo, images, ownerID, in.KeyboardID, onDelete)
		if blocked, ok := errors.AsType[*cascadedelete.BlockedError](err); ok {
			return nil, schema.DeleteKeyboardOutput{}, fmt.Errorf("keyboard is still referenced by builds: %s", strings.Join(blocked.BuildIDs, ", "))
		}
		if err != nil {
			log.FromContext(ctx).Error("deleting keyboard", log.KeyboardID, in.KeyboardID, log.Error, err)
			return nil, schema.DeleteKeyboardOutput{}, errors.New("failed to delete keyboard")
		}

		return nil, schema.DeleteKeyboardOutput{DeletedBuildIDs: result.DeletedBuildIDs}, nil
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
// handlers.keyboardFieldErrorToInvalidParam.
func keyboardFieldErrorReason(fe lookup.FieldError, size *string) string {
	if fe.Field == "layout" && fe.Category == lookup.CategoryKeyboardSize && size != nil {
		return fmt.Sprintf("layout: %q is not a valid layout for size %q", fe.Value, *size)
	}

	return fmt.Sprintf("%s: %q is not an approved %s value", fe.Field, fe.Value, fe.Category)
}
