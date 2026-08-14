package cascadedelete

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// DeleteSwitch deletes the switch identified by (ownerID, switchID),
// applying onDelete's policy toward any build that still references it:
//
//   - OnDeleteBlock: if any build references switchID, deletes nothing and
//     returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing switchID first, then
//     the switch. Returns the deleted build ids.
//   - OnDeleteDetach: deletes the switch unconditionally, without even
//     checking for references, leaving any referencing build's
//     switches[].switch field dangling.
//
// onDelete must be one of the three OnDeleteX values - see [ParseOnDelete].
// DeleteSwitch does no authorization itself; ownerID must already be the
// caller's own resolved subject.
func DeleteSwitch(
	ctx context.Context,
	switchRepo repository.SwitchRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	ownerID, switchID string,
	onDelete OnDelete,
) (Result, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade, OnDeleteDetach:
	default:
		return Result{}, fmt.Errorf("deleting switch %q: unknown on_delete value %q", switchID, onDelete)
	}

	if onDelete == OnDeleteDetach {
		if err := switchRepo.Delete(ctx, switchID); err != nil {
			return Result{}, fmt.Errorf("detach-deleting switch %q: %w", switchID, err)
		}
		return Result{}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingSwitch(ctx, ownerID, switchID)
	if err != nil {
		return Result{}, fmt.Errorf("finding builds referencing switch %q: %w", switchID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return Result{}, &BlockedError{BuildIDs: buildIDs}
		}
		if err := switchRepo.Delete(ctx, switchID); err != nil {
			return Result{}, fmt.Errorf("deleting unreferenced switch %q: %w", switchID, err)
		}
		return Result{}, nil
	}

	for _, buildID := range buildIDs {
		imageKeys, err := buildRepo.Delete(ctx, buildID)
		if err != nil {
			return Result{}, fmt.Errorf("cascade-deleting build %q referencing switch %q: %w", buildID, switchID, err)
		}
		images.BestEffortDelete(ctx, imageKeys)
	}

	if err := switchRepo.Delete(ctx, switchID); err != nil {
		return Result{}, fmt.Errorf("deleting switch %q after cascading %d build(s): %w", switchID, len(buildIDs), err)
	}

	return Result{DeletedBuildIDs: buildIDs}, nil
}
