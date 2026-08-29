package repoapi

import (
	"context"
	"errors"
	"fmt"
	"sync"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// BuildToAPI maps a repository.Build to its wire representation, resolving
// the Keyboard/Switch/KeycapSet references it carries into denormalized
// objects so a client can render the build without follow-up requests. A
// reference that can't be resolved (repository.ErrNotFound - e.g. deleted
// after the build referenced it, see
// https://github.com/rogueserenity/kbdb/issues/172) is left nil rather than
// failing the whole request, mirroring [BuildToAPISummary]; any other
// repository error, or b.BuildDate not matching dateLayout, or an image
// failing to presign, still fails it.
//
// isOwner hides Stabs.Price and TotalCost from non-owners.
func BuildToAPI(
	ctx context.Context, b repository.Build,
	images repository.BuildImageStore,
	kitImages repository.KeycapKitImageStore,
	keyboardImages repository.KeyboardImageStore,
	switchImages repository.SwitchImageStore,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
	isOwner bool,
) (api.Build, error) {
	buildDate, err := buildDateToAPI(b.BuildDate)
	if err != nil {
		return api.Build{}, err
	}

	imgs, err := buildImagesToAPI(ctx, b.Images, images)
	if err != nil {
		return api.Build{}, err
	}

	keyboardRef, keyboardPrice, err := buildKeyboardRefToAPI(ctx, b.UserID, b.Keyboard, keyboardRepo, keyboardImages, true)
	if err != nil {
		return api.Build{}, err
	}

	switches, switchesCost, err := buildSwitchEntriesResolvedToAPI(ctx, b.UserID, b.Switches, switchRepo, switchImages, true)
	if err != nil {
		return api.Build{}, err
	}

	keycapKits, keycapKitsCost, err := buildKeycapKitEntriesResolvedToAPI(ctx, b.UserID, b.KeycapKits, keycapSetRepo, kitImages, true)
	if err != nil {
		return api.Build{}, err
	}

	out := api.Build{
		Id:            b.ID,
		Keyboard:      keyboardRef,
		Plate:         b.Plate,
		CaseMountType: buildCaseMountTypeToAPI(b.CaseMountType),
		Stabs:         buildStabsToAPI(b.Stabs, isOwner),
		Foam:          b.Foam,
		Switches:      switches,
		KeycapKits:    keycapKits,
		BuildDate:     buildDate,
		Notes:         b.Notes,
		Visibility:    api.Visibility(b.Visibility),
		Images:        imgs,
	}

	if isOwner {
		var stabsPrice *float64
		if b.Stabs != nil {
			stabsPrice = b.Stabs.Price
		}
		out.TotalCost = sumKnownCosts(keyboardPrice, switchesCost, keycapKitsCost, stabsPrice)
	}

	return out, nil
}

// sumKnownCosts sums the non-nil components, treating a nil one as
// excluded rather than zero. Returns nil if none are set.
func sumKnownCosts(components ...*float64) *float64 {
	var total float64
	var haveAny bool
	for _, c := range components {
		if c == nil {
			continue
		}
		total += *c
		haveAny = true
	}

	if !haveAny {
		return nil //nolint:nilnil // no known-priced components is a valid, expected result
	}

	return &total
}

// BuildToRepo maps a generated BuildInput (already schema-validated by the
// OpenAPI request validator) to a repository.Build. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility. Images is never set here - a build's images
// are managed entirely through the dedicated image endpoints, never carried
// in a build write.
func BuildToRepo(in api.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		Plate:         in.Plate,
		CaseMountType: buildCaseMountTypeToRepo(in.CaseMountType),
		Stabs:         buildStabsToRepo(in.Stabs),
		Foam:          in.Foam,
		Switches:      buildSwitchEntriesToRepo(in.Switches),
		KeycapKits:    buildKeycapKitEntriesToRepo(in.KeycapKits),
		BuildDate:     buildDateToRepo(in.BuildDate),
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// BuildToAPISummary denormalizes the referenced Keyboard's brand/name via
// a per-item keyboardRepo.Get call - no batch-get precedent exists, and a
// list page is capped at 100 items, so this O(n) fetch is the simplest
// correct approach for now. If the keyboard can't be resolved
// (repository.ErrNotFound - e.g. deleted after the build was created, see
// https://github.com/rogueserenity/kbdb/issues/172), Keyboard is left nil
// rather than failing the whole request; any other error still fails it.
//
// TotalCost mirrors [BuildToAPI]'s calculation and isOwner gating; unlike
// keyboardPrice, switches/keycap kits are only resolved when isOwner,
// since cost is the only thing this uses them for.
func BuildToAPISummary(
	ctx context.Context, b repository.Build,
	keyboardRepo repository.KeyboardRepository,
	switchRepo repository.SwitchRepository,
	keycapSetRepo repository.KeycapSetRepository,
	images repository.BuildImageStore,
	isOwner bool,
) (api.BuildSummary, error) {
	buildDate, err := buildDateToAPI(b.BuildDate)
	if err != nil {
		return api.BuildSummary{}, err
	}

	var image *api.BuildImage
	if len(b.Images) > 0 {
		url, err := images.PresignGetBuildImage(ctx, b.Images[0].Path)
		if err != nil {
			return api.BuildSummary{}, fmt.Errorf("presigning build image %q: %w", b.Images[0].ImageID, err)
		}
		image = &api.BuildImage{ImageId: b.Images[0].ImageID, Url: url}
	}

	summary := api.BuildSummary{
		Id:        &b.ID,
		BuildDate: buildDate,
		Image:     image,
	}

	kb, keyboardPrice, err := buildKeyboardRefToAPI(ctx, b.UserID, b.Keyboard, keyboardRepo, nil, false)
	if err != nil {
		return api.BuildSummary{}, err
	}
	if kb != nil {
		summary.Keyboard = &api.BuildSummaryKeyboard{Brand: &kb.Brand, Name: &kb.Name}
	}

	if isOwner {
		_, switchesCost, err := buildSwitchEntriesResolvedToAPI(ctx, b.UserID, b.Switches, switchRepo, nil, false)
		if err != nil {
			return api.BuildSummary{}, err
		}

		_, keycapKitsCost, err := buildKeycapKitEntriesResolvedToAPI(ctx, b.UserID, b.KeycapKits, keycapSetRepo, nil, false)
		if err != nil {
			return api.BuildSummary{}, err
		}

		var stabsPrice *float64
		if b.Stabs != nil {
			stabsPrice = b.Stabs.Price
		}
		summary.TotalCost = sumKnownCosts(keyboardPrice, switchesCost, keycapKitsCost, stabsPrice)
	}

	return summary, nil
}

func buildDateToAPI(s *string) (*openapi_types.Date, error) {
	if s == nil {
		return nil, nil //nolint:nilnil // no build date is a valid, expected result
	}

	d, err := parseAPIDate(*s)
	if err != nil {
		return nil, fmt.Errorf("parsing build_date: %w", err)
	}

	return d, nil
}

func buildDateToRepo(d *openapi_types.Date) *string {
	if d == nil {
		return nil
	}

	s := d.Format(dateLayout)
	return &s
}

func buildCaseMountTypeToAPI(cmt *repository.BuildCaseMountType) *api.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &api.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildCaseMountTypeToRepo(cmt *api.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildStabsToAPI(s *repository.BuildStabs, isOwner bool) *api.BuildStabs {
	if s == nil {
		return nil
	}

	out := &api.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
	}
	if isOwner {
		out.Price = s.Price
	}

	return out
}

func buildStabsToRepo(s *api.BuildStabs) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func buildSwitchEntriesToRepo(entries *[]api.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func buildKeycapKitEntriesToRepo(entries *[]api.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

// buildKeyboardRefToAPI resolves keyboardID into a denormalized reference
// plus its purchase price. Returns (nil, nil, nil) if the keyboard no
// longer exists.
//
// resolveImages false skips presigning ImageUrl; keyboardImages may be nil
// in that case. When true, the ref surfaces only the keyboard's first
// image (kb.Images[0]), mirroring how [BuildToAPISummary] picks a build's
// own Images[0] for its summary thumbnail.
func buildKeyboardRefToAPI(
	ctx context.Context, ownerID, keyboardID string,
	keyboardRepo repository.KeyboardRepository, keyboardImages repository.KeyboardImageStore,
	resolveImages bool,
) (*api.BuildKeyboardRef, *float64, error) {
	kb, err := keyboardRepo.Get(ctx, ownerID, keyboardID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, nil //nolint:nilnil // deleted-after-reference is a valid, expected result
		}
		return nil, nil, fmt.Errorf("getting keyboard %q: %w", keyboardID, err)
	}

	ref := &api.BuildKeyboardRef{
		Id:     kb.ID,
		Brand:  kb.Brand,
		Name:   kb.Name,
		Size:   kb.Size,
		Layout: kb.Layout,
	}

	if resolveImages && len(kb.Images) > 0 {
		url, err := keyboardImages.PresignGetKeyboardImage(ctx, kb.Images[0].Path)
		if err != nil {
			return nil, nil, fmt.Errorf("presigning keyboard image for keyboard %q: %w", keyboardID, err)
		}
		ref.ImageUrl = &url
	}

	return ref, kb.Purchase.Price, nil
}

// buildSwitchEntriesResolvedToAPI resolves each entry's Switch id into a
// denormalized reference via a per-entry switchRepo.Get call - same
// per-item-fetch approach as [BuildToAPISummary]'s keyboard lookup, run
// concurrently across entries since each only touches its own out[i]. An
// entry whose switch no longer exists keeps its Count but leaves Switch
// nil rather than dropping the entry or failing the request. Also returns
// the summed cost across entries with a known per-unit price: switches are
// bought in bulk (SwitchPurchase.Price is the total for Quantity units, not
// a per-unit price - see SwitchPurchase's doc), so an entry contributes
// (Price/Quantity)*Count only when Quantity is set and non-zero; otherwise
// its cost is unknown and excluded rather than guessed at.
//
// resolveImages false skips presigning ImageUrl; switchImages may be nil in
// that case.
func buildSwitchEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildSwitchEntry,
	switchRepo repository.SwitchRepository, switchImages repository.SwitchImageStore,
	resolveImages bool,
) (*[]api.BuildSwitchEntryResolved, *float64, error) {
	if entries == nil {
		return nil, nil, nil //nolint:nilnil // no switches is a valid, expected result
	}

	out := make([]api.BuildSwitchEntryResolved, len(entries))
	costs := make([]*float64, len(entries))
	errs := make([]error, len(entries))

	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(i int, e repository.BuildSwitchEntry) {
			defer wg.Done()

			sw, err := switchRepo.Get(ctx, ownerID, e.Switch)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					errs[i] = fmt.Errorf("getting switch %q: %w", e.Switch, err)
					return
				}
				out[i] = api.BuildSwitchEntryResolved{Count: e.Count}
				return
			}

			out[i] = api.BuildSwitchEntryResolved{
				Count: e.Count,
				Switch: &api.BuildSwitchRef{
					Id:           sw.ID,
					Brand:        sw.Brand,
					Manufacturer: sw.Manufacturer,
					Name:         sw.Name,
					Type:         sw.Type,
				},
			}

			if resolveImages && sw.ImagePath != nil {
				url, err := switchImages.PresignGet(ctx, *sw.ImagePath)
				if err != nil {
					errs[i] = fmt.Errorf("presigning switch image for switch %q: %w", e.Switch, err)
					return
				}
				out[i].Switch.ImageUrl = &url
			}

			if sw.Purchase.Price != nil && sw.Purchase.Quantity != nil && *sw.Purchase.Quantity != 0 {
				unitPrice := *sw.Purchase.Price / float64(*sw.Purchase.Quantity)
				entryCost := unitPrice * float64(e.Count)
				costs[i] = &entryCost
			}
		}(i, e)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return &out, sumKnownCosts(costs...), nil
}

// buildKeycapKitEntriesResolvedToAPI resolves each entry's (KeycapSet, Kit)
// pair into a denormalized reference plus the kit's own name/image, via a
// per-entry keycapSetRepo.Get call - same per-item-fetch approach as
// [buildSwitchEntriesResolvedToAPI], run concurrently across entries for the
// same reason. An entry whose keycap set - or whose kit within it - no
// longer exists keeps its KitId but leaves KeycapSet, KitName, and
// KitImageUrl nil rather than dropping the entry or failing the request.
// Also returns the summed price across entries whose kit has a known
// price.
//
// resolveImages false skips presigning KitImageUrl; kitImages may be nil
// in that case.
func buildKeycapKitEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildKeycapKitEntry,
	keycapSetRepo repository.KeycapSetRepository, kitImages repository.KeycapKitImageStore,
	resolveImages bool,
) (*[]api.BuildKeycapKitEntryResolved, *float64, error) {
	if entries == nil {
		return nil, nil, nil //nolint:nilnil // no keycap kits is a valid, expected result
	}

	out := make([]api.BuildKeycapKitEntryResolved, len(entries))
	costs := make([]*float64, len(entries))
	errs := make([]error, len(entries))

	var wg sync.WaitGroup
	for i, e := range entries {
		wg.Add(1)
		go func(i int, e repository.BuildKeycapKitEntry) {
			defer wg.Done()

			out[i] = api.BuildKeycapKitEntryResolved{KitId: e.Kit}

			ks, err := keycapSetRepo.Get(ctx, ownerID, e.KeycapSet)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					errs[i] = fmt.Errorf("getting keycap set %q: %w", e.KeycapSet, err)
				}
				return
			}

			kit := findKeycapKit(ks.Kits, e.Kit)
			if kit == nil {
				return
			}

			out[i].KeycapSet = &api.BuildKeycapSetRef{
				Id:      ks.ID,
				Brand:   ks.Brand,
				Name:    ks.Name,
				Profile: ks.Profile,
			}
			out[i].KitName = &kit.Name
			costs[i] = kit.Purchase.Price

			if resolveImages && kit.ImagePath != nil {
				url, err := kitImages.PresignGet(ctx, *kit.ImagePath)
				if err != nil {
					errs[i] = fmt.Errorf("presigning kit image for kit %q: %w", e.Kit, err)
					return
				}
				out[i].KitImageUrl = &url
			}
		}(i, e)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, nil, err
	}

	return &out, sumKnownCosts(costs...), nil
}

func findKeycapKit(kits map[string]repository.KeycapKit, kitID string) *repository.KeycapKit {
	kit, ok := kits[kitID]
	if !ok {
		return nil
	}
	return &kit
}

// buildImagesToAPI mints a fresh presigned GET URL per image, per request -
// never persisted, mirroring [KeycapKitToAPI]'s handling of a kit's image.
func buildImagesToAPI(ctx context.Context, images []repository.BuildImage, store repository.BuildImageStore) (*[]api.BuildImage, error) {
	if images == nil {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.BuildImage, len(images))
	errs := make([]error, len(images))

	var wg sync.WaitGroup
	for i, img := range images {
		wg.Add(1)
		go func(i int, img repository.BuildImage) {
			defer wg.Done()

			url, err := store.PresignGetBuildImage(ctx, img.Path)
			if err != nil {
				errs[i] = fmt.Errorf("presigning build image %q: %w", img.ImageID, err)
				return
			}
			out[i] = api.BuildImage{ImageId: img.ImageID, Url: url}
		}(i, img)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &out, nil
}
