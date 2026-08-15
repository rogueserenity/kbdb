package cascadedelete

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapKitResult is a DeleteKeycapKit call's success return value.
// ImageKey is the image key that was on the deleted kit, or nil if it had
// none - callers clean it up in a KeycapKitImageStore themselves, same as
// before this package existed. DeletedBuildIDs (via the embedded Result) is
// empty except in cascade mode.
type KeycapKitResult struct {
	Result
	ImageKey *repository.KeycapKitImageKey
}

// DeleteKeycapKit deletes the kit identified by (setID, kitID) from the
// caller's keycap set, applying onDelete's policy toward any build that
// still references it:
//
//   - OnDeleteBlock: if any build references (setID, kitID), deletes
//     nothing and returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing (setID, kitID)
//     first, then the kit. Returns the deleted build ids.
//   - OnDeleteDetach: deletes the kit unconditionally, without even
//     checking for references, leaving any referencing build's
//     keycap_kits[] entry dangling.
//
// onDelete must be one of the three OnDeleteX values - see [ParseOnDelete].
// DeleteKeycapKit does no authorization itself; ownerID must already be the
// caller's own resolved subject.
func DeleteKeycapKit(
	ctx context.Context,
	keycapSetRepo repository.KeycapSetRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	ownerID, setID, kitID string,
	onDelete OnDelete,
) (KeycapKitResult, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade, OnDeleteDetach:
	default:
		return KeycapKitResult{}, fmt.Errorf("deleting keycap kit %q/%q: unknown on_delete value %q", setID, kitID, onDelete)
	}

	if onDelete == OnDeleteDetach {
		cleared, err := keycapSetRepo.DeleteKit(ctx, setID, kitID)
		if err != nil {
			return KeycapKitResult{}, fmt.Errorf("detach-deleting keycap kit %q/%q: %w", setID, kitID, err)
		}
		return KeycapKitResult{ImageKey: cleared}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingKeycapKit(ctx, ownerID, setID, kitID)
	if err != nil {
		return KeycapKitResult{}, fmt.Errorf("finding builds referencing keycap kit %q/%q: %w", setID, kitID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return KeycapKitResult{}, &BlockedError{BuildIDs: buildIDs}
		}
		cleared, err := keycapSetRepo.DeleteKit(ctx, setID, kitID)
		if err != nil {
			return KeycapKitResult{}, fmt.Errorf("deleting unreferenced keycap kit %q/%q: %w", setID, kitID, err)
		}
		return KeycapKitResult{ImageKey: cleared}, nil
	}

	for _, buildID := range buildIDs {
		imageKeys, err := buildRepo.Delete(ctx, buildID)
		if err != nil {
			return KeycapKitResult{}, fmt.Errorf("cascade-deleting build %q referencing keycap kit %q/%q: %w", buildID, setID, kitID, err)
		}
		images.BestEffortDelete(ctx, imageKeys)
	}

	cleared, err := keycapSetRepo.DeleteKit(ctx, setID, kitID)
	if err != nil {
		return KeycapKitResult{}, fmt.Errorf("deleting keycap kit %q/%q after cascading %d build(s): %w", setID, kitID, len(buildIDs), err)
	}

	return KeycapKitResult{Result: Result{DeletedBuildIDs: buildIDs}, ImageKey: cleared}, nil
}
