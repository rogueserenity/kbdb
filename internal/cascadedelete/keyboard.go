package cascadedelete

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeyboardResult is a DeleteKeyboard call's success return value.
// ImageKeys are the image keys the deleted keyboard had, or nil if it had
// none - callers clean them up in a KeyboardImageStore themselves.
// DeletedBuildIDs (via the embedded Result) is empty except in cascade
// mode.
type KeyboardResult struct {
	Result
	ImageKeys []repository.KeyboardImageKey
}

// DeleteKeyboard deletes the keyboard identified by (ownerID, keyboardID),
// applying onDelete's policy toward any build that still references it:
//
//   - OnDeleteBlock: if any build references keyboardID, deletes nothing
//     and returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing keyboardID first,
//     then the keyboard. Returns the deleted build ids.
//   - OnDeleteDetach: deletes the keyboard unconditionally, without even
//     checking for references, leaving any referencing build's keyboard
//     field dangling.
//
// onDelete must be one of the three OnDeleteX values - see [ParseOnDelete].
// DeleteKeyboard does no authorization itself; ownerID must already be the
// caller's own resolved subject.
func DeleteKeyboard(
	ctx context.Context,
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	ownerID, keyboardID string,
	onDelete OnDelete,
) (KeyboardResult, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade, OnDeleteDetach:
	default:
		return KeyboardResult{}, fmt.Errorf("deleting keyboard %q: unknown on_delete value %q", keyboardID, onDelete)
	}

	if onDelete == OnDeleteDetach {
		imageKeys, err := keyboardRepo.Delete(ctx, keyboardID)
		if err != nil {
			return KeyboardResult{}, fmt.Errorf("detach-deleting keyboard %q: %w", keyboardID, err)
		}
		return KeyboardResult{ImageKeys: imageKeys}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingKeyboard(ctx, ownerID, keyboardID)
	if err != nil {
		return KeyboardResult{}, fmt.Errorf("finding builds referencing keyboard %q: %w", keyboardID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return KeyboardResult{}, &BlockedError{BuildIDs: buildIDs}
		}
		imageKeys, err := keyboardRepo.Delete(ctx, keyboardID)
		if err != nil {
			return KeyboardResult{}, fmt.Errorf("deleting unreferenced keyboard %q: %w", keyboardID, err)
		}
		return KeyboardResult{ImageKeys: imageKeys}, nil
	}

	for _, buildID := range buildIDs {
		imageKeys, err := buildRepo.Delete(ctx, buildID)
		if err != nil {
			return KeyboardResult{}, fmt.Errorf("cascade-deleting build %q referencing keyboard %q: %w", buildID, keyboardID, err)
		}
		images.BestEffortDelete(ctx, imageKeys)
	}

	imageKeys, err := keyboardRepo.Delete(ctx, keyboardID)
	if err != nil {
		return KeyboardResult{}, fmt.Errorf("deleting keyboard %q after cascading %d build(s): %w", keyboardID, len(buildIDs), err)
	}

	return KeyboardResult{Result: Result{DeletedBuildIDs: buildIDs}, ImageKeys: imageKeys}, nil
}
