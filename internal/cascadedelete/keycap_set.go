package cascadedelete

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetResult is a DeleteKeycapSet call's success return value.
// ImageKeys are the image keys the deleted set's kits had, or nil if none
// did - callers clean them up in a KeycapKitImageStore themselves, same as
// before this package existed. DeletedBuildIDs (via the embedded Result) is
// empty except in cascade mode.
type KeycapSetResult struct {
	Result
	ImageKeys []repository.KeycapKitImageKey
}

// DeleteKeycapSet deletes the keycap set identified by (ownerID, setID),
// applying onDelete's policy toward any build that still references any
// kit in the set:
//
//   - OnDeleteBlock: if any build references any kit in setID, deletes
//     nothing and returns a *BlockedError listing those build ids.
//   - OnDeleteCascade: deletes every build referencing any kit in setID
//     first, then the set. Returns the deleted build ids.
//   - OnDeleteDetach: deletes the set unconditionally, without even
//     checking for references, leaving any referencing build's
//     keycap_kits[] entry dangling.
//
// onDelete must be one of the three OnDeleteX values - see [ParseOnDelete].
// DeleteKeycapSet does no authorization itself; ownerID must already be the
// caller's own resolved subject.
func DeleteKeycapSet(
	ctx context.Context,
	keycapSetRepo repository.KeycapSetRepository,
	buildRepo repository.BuildRepository,
	images repository.BuildImageStore,
	ownerID, setID string,
	onDelete OnDelete,
) (KeycapSetResult, error) {
	switch onDelete {
	case OnDeleteBlock, OnDeleteCascade, OnDeleteDetach:
	default:
		return KeycapSetResult{}, fmt.Errorf("deleting keycap set %q: unknown on_delete value %q", setID, onDelete)
	}

	if onDelete == OnDeleteDetach {
		imageKeys, err := keycapSetRepo.Delete(ctx, setID)
		if err != nil {
			return KeycapSetResult{}, fmt.Errorf("detach-deleting keycap set %q: %w", setID, err)
		}
		return KeycapSetResult{ImageKeys: imageKeys}, nil
	}

	buildIDs, err := buildRepo.FindBuildsReferencingKeycapSet(ctx, ownerID, setID)
	if err != nil {
		return KeycapSetResult{}, fmt.Errorf("finding builds referencing keycap set %q: %w", setID, err)
	}

	if onDelete == OnDeleteBlock {
		if len(buildIDs) > 0 {
			return KeycapSetResult{}, &BlockedError{BuildIDs: buildIDs}
		}
		imageKeys, err := keycapSetRepo.Delete(ctx, setID)
		if err != nil {
			return KeycapSetResult{}, fmt.Errorf("deleting unreferenced keycap set %q: %w", setID, err)
		}
		return KeycapSetResult{ImageKeys: imageKeys}, nil
	}

	errs := make([]error, len(buildIDs))
	var wg sync.WaitGroup
	for i, buildID := range buildIDs {
		wg.Add(1)
		go func(i int, buildID string) {
			defer wg.Done()

			imageKeys, err := buildRepo.Delete(ctx, buildID)
			if err != nil {
				errs[i] = fmt.Errorf("cascade-deleting build %q referencing keycap set %q: %w", buildID, setID, err)
				return
			}
			images.BestEffortDelete(ctx, imageKeys)
		}(i, buildID)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return KeycapSetResult{}, err
	}

	imageKeys, err := keycapSetRepo.Delete(ctx, setID)
	if err != nil {
		return KeycapSetResult{}, fmt.Errorf("deleting keycap set %q after cascading %d build(s): %w", setID, len(buildIDs), err)
	}

	return KeycapSetResult{Result: Result{DeletedBuildIDs: buildIDs}, ImageKeys: imageKeys}, nil
}
