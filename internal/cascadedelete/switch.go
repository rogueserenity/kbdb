package cascadedelete

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// SwitchResult is a DeleteSwitch call's success return value. ImageKey is
// the image key the deleted switch had, or nil if it had none - callers
// clean it up in a SwitchImageStore themselves. DeletedBuildIDs (via the
// embedded Result) is empty except in cascade mode.
type SwitchResult struct {
	Result
	ImageKey *repository.SwitchImageKey
}

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
) (SwitchResult, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade, OnDeleteDetach:
	default:
		return SwitchResult{}, fmt.Errorf("deleting switch %q: unknown on_delete value %q", switchID, onDelete)
	}

	if onDelete == OnDeleteDetach {
		imageKey, err := switchRepo.Delete(ctx, switchID)
		if err != nil {
			return SwitchResult{}, fmt.Errorf("detach-deleting switch %q: %w", switchID, err)
		}
		return SwitchResult{ImageKey: imageKey}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingSwitch(ctx, ownerID, switchID)
	if err != nil {
		return SwitchResult{}, fmt.Errorf("finding builds referencing switch %q: %w", switchID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return SwitchResult{}, &BlockedError{BuildIDs: buildIDs}
		}
		imageKey, err := switchRepo.Delete(ctx, switchID)
		if err != nil {
			return SwitchResult{}, fmt.Errorf("deleting unreferenced switch %q: %w", switchID, err)
		}
		return SwitchResult{ImageKey: imageKey}, nil
	}

	errs := make([]error, len(buildIDs))
	var wg sync.WaitGroup
	for i, buildID := range buildIDs {
		wg.Add(1)
		go func(i int, buildID string) {
			defer wg.Done()

			imageKeys, err := buildRepo.Delete(ctx, buildID)
			if err != nil {
				errs[i] = fmt.Errorf("cascade-deleting build %q referencing switch %q: %w", buildID, switchID, err)
				return
			}
			images.BestEffortDelete(ctx, imageKeys)
		}(i, buildID)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return SwitchResult{}, err
	}

	imageKey, err := switchRepo.Delete(ctx, switchID)
	if err != nil {
		return SwitchResult{}, fmt.Errorf("deleting switch %q after cascading %d build(s): %w", switchID, len(buildIDs), err)
	}

	return SwitchResult{Result: Result{DeletedBuildIDs: buildIDs}, ImageKey: imageKey}, nil
}
