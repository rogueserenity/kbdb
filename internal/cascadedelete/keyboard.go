package cascadedelete

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// DeleteKeyboard deletes the keyboard identified by (ownerID, keyboardID),
// applying onDelete's policy toward any build that still references it:
//
//   - OnDeleteBlock (the default): if any build references keyboardID,
//     deletes nothing and returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing keyboardID first,
//     then the keyboard. Returns the deleted build ids.
//   - OnDeleteDetach: deletes the keyboard unconditionally, without even
//     checking for references, leaving any referencing build's keyboard
//     field dangling.
//
// DeleteKeyboard does no authorization itself; ownerID must already be the
// caller's own resolved subject.
func DeleteKeyboard(
	ctx context.Context,
	keyboardRepo repository.KeyboardRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	ownerID, keyboardID string,
	onDelete OnDelete,
) (Result, error) {
	if onDelete == OnDeleteDetach {
		if err := keyboardRepo.Delete(ctx, keyboardID); err != nil {
			return Result{}, fmt.Errorf("detach-deleting keyboard %q: %w", keyboardID, err)
		}
		return Result{}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingKeyboard(ctx, ownerID, keyboardID)
	if err != nil {
		return Result{}, fmt.Errorf("finding builds referencing keyboard %q: %w", keyboardID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return Result{}, &BlockedError{BuildIDs: buildIDs}
		}
		if err := keyboardRepo.Delete(ctx, keyboardID); err != nil {
			return Result{}, fmt.Errorf("deleting unreferenced keyboard %q: %w", keyboardID, err)
		}
		return Result{}, nil
	}

	for _, buildID := range buildIDs {
		imageKeys, err := buildRepo.Delete(ctx, buildID)
		if err != nil {
			return Result{}, fmt.Errorf("cascade-deleting build %q referencing keyboard %q: %w", buildID, keyboardID, err)
		}
		images.BestEffortDelete(ctx, imageKeys)
	}

	if err := keyboardRepo.Delete(ctx, keyboardID); err != nil {
		return Result{}, fmt.Errorf("deleting keyboard %q after cascading %d build(s): %w", keyboardID, len(buildIDs), err)
	}

	return Result{DeletedBuildIDs: buildIDs}, nil
}
