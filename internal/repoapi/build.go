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

// Build maps repository.Build to and from its wire representations,
// resolving the Keyboard/Switch/KeycapSet references it carries.
type Build struct {
	Repo           repository.BuildRepository
	Images         repository.BuildImageStore
	KitImages      repository.KeycapKitImageStore
	KeyboardImages repository.KeyboardImageStore
	SwitchImages   repository.SwitchImageStore
	KeyboardRepo   repository.KeyboardRepository
	SwitchRepo     repository.SwitchRepository
	KeycapSetRepo  repository.KeycapSetRepository
}

// ToAPI maps a repository.Build to its wire representation, resolving the
// Keyboard/Switch/KeycapSet references it carries into denormalized
// objects so a client can render the build without follow-up requests. A
// reference that can't be resolved (repository.ErrNotFound - e.g. deleted
// after the build referenced it, see
// https://github.com/rogueserenity/kbdb/issues/172) is left nil rather than
// failing the whole request, mirroring [Build.ToAPISummary]; any other
// repository error, or b.BuildDate not matching dateLayout, or an image
// failing to presign, still fails it.
//
// The owner always sees their own Stabs.Price and TotalCost; a non-owner
// sees them only if ownerPrefs.ShowPriceToOthers. A build's Keyboard,
// Switches, and KeycapKits all belong to the same owner as the build
// itself, so ownerPrefs (looked up once for the build's owner) governs
// price visibility across the whole composite.
func (b Build) ToAPI(ctx context.Context, build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Build, error) {
	showPrice := ownerPrefs.ShowPriceSingle(isOwner)
	buildDate, err := b.dateToAPI(build.BuildDate)
	if err != nil {
		return api.Build{}, err
	}

	imgs, err := b.imagesToAPI(ctx, repository.SortedBuildImages(build.Images))
	if err != nil {
		return api.Build{}, err
	}

	keyboardRef, keyboardPrice, err := b.keyboardRefToAPI(ctx, build.UserID, build.Keyboard, true)
	if err != nil {
		return api.Build{}, err
	}

	switches, switchesCost, err := b.switchEntriesResolvedToAPI(ctx, build.UserID, build.Switches, true)
	if err != nil {
		return api.Build{}, err
	}

	keycapKits, keycapKitsCost, err := b.keycapKitEntriesResolvedToAPI(ctx, build.UserID, build.KeycapKits, true)
	if err != nil {
		return api.Build{}, err
	}

	out := api.Build{
		Id:            build.ID,
		Keyboard:      keyboardRef,
		Plate:         build.Plate,
		CaseMountType: b.caseMountTypeToAPI(build.CaseMountType),
		Stabs:         b.stabsToAPI(build.Stabs, showPrice),
		Foam:          build.Foam,
		Switches:      switches,
		KeycapKits:    keycapKits,
		BuildDate:     buildDate,
		Notes:         build.Notes,
		Visibility:    api.Visibility(build.Visibility),
		Images:        imgs,
	}

	if showPrice {
		var stabsPrice *float64
		if build.Stabs != nil {
			stabsPrice = build.Stabs.Price
		}
		out.TotalCost = sumKnownCosts(keyboardPrice, switchesCost, keycapKitsCost, stabsPrice)
	}

	return out, nil
}

// ToRepo maps a generated BuildInput (already schema-validated by the
// OpenAPI request validator) to a repository.Build. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility. Images is never set here - a build's images
// are managed entirely through the dedicated image endpoints, never carried
// in a build write.
func (b Build) ToRepo(in api.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		Plate:         in.Plate,
		CaseMountType: b.caseMountTypeToRepo(in.CaseMountType),
		Stabs:         b.stabsToRepo(in.Stabs),
		Foam:          in.Foam,
		Switches:      b.switchEntriesToRepo(in.Switches),
		KeycapKits:    b.keycapKitEntriesToRepo(in.KeycapKits),
		BuildDate:     b.dateToRepo(in.BuildDate),
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// ToAPISummary denormalizes the referenced Keyboard's brand/name via a
// per-item KeyboardRepo.Get call - no batch-get precedent exists, and a
// list page is capped at 100 items, so this O(n) fetch is the simplest
// correct approach for now. If the keyboard can't be resolved
// (repository.ErrNotFound - e.g. deleted after the build was created, see
// https://github.com/rogueserenity/kbdb/issues/172), Keyboard is left nil
// rather than failing the whole request; any other error still fails it.
//
// TotalCost mirrors [Build.ToAPI]'s calculation, gated by
// ownerPrefs.ShowPriceSummary(isOwner) instead of ShowPriceSingle - unlike
// [Build.ToAPI], the owner isn't unconditionally shown price here. Unlike
// keyboardPrice, switches/keycap kits are only resolved when price will be
// shown, since cost is the only thing this uses them for.
func (b Build) ToAPISummary(ctx context.Context, build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.BuildSummary, error) {
	showPrice := ownerPrefs.ShowPriceSummary(isOwner)
	buildDate, err := b.dateToAPI(build.BuildDate)
	if err != nil {
		return api.BuildSummary{}, err
	}

	var image *api.BuildImage
	if imgs := repository.SortedBuildImages(build.Images); len(imgs) > 0 {
		url, err := b.Images.PresignGetBuildImage(ctx, imgs[0].Path)
		if err != nil {
			return api.BuildSummary{}, fmt.Errorf("presigning build image %q: %w", imgs[0].ImageID, err)
		}
		image = &api.BuildImage{ImageId: imgs[0].ImageID, Url: url}
	}

	summary := api.BuildSummary{
		Id:         &build.ID,
		KeyboardId: &build.Keyboard,
		BuildDate:  buildDate,
		Image:      image,
	}

	kb, keyboardPrice, err := b.keyboardRefToAPI(ctx, build.UserID, build.Keyboard, false)
	if err != nil {
		return api.BuildSummary{}, err
	}
	if kb != nil {
		summary.Keyboard = &api.BuildSummaryKeyboard{Brand: &kb.Brand, Name: &kb.Name}
	}

	if showPrice {
		_, switchesCost, err := b.switchEntriesResolvedToAPI(ctx, build.UserID, build.Switches, false)
		if err != nil {
			return api.BuildSummary{}, err
		}

		_, keycapKitsCost, err := b.keycapKitEntriesResolvedToAPI(ctx, build.UserID, build.KeycapKits, false)
		if err != nil {
			return api.BuildSummary{}, err
		}

		var stabsPrice *float64
		if build.Stabs != nil {
			stabsPrice = build.Stabs.Price
		}
		summary.TotalCost = sumKnownCosts(keyboardPrice, switchesCost, keycapKitsCost, stabsPrice)
	}

	return summary, nil
}

func (b Build) dateToAPI(s *string) (*openapi_types.Date, error) {
	if s == nil {
		return nil, nil //nolint:nilnil // no build date is a valid, expected result
	}

	d, err := parseAPIDate(*s)
	if err != nil {
		return nil, fmt.Errorf("parsing build_date: %w", err)
	}

	return d, nil
}

func (b Build) dateToRepo(d *openapi_types.Date) *string {
	if d == nil {
		return nil
	}

	s := d.Format(dateLayout)
	return &s
}

func (b Build) caseMountTypeToAPI(cmt *repository.BuildCaseMountType) *api.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &api.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) caseMountTypeToRepo(cmt *api.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) stabsToAPI(s *repository.BuildStabs, showPrice bool) *api.BuildStabs {
	if s == nil {
		return nil
	}

	out := &api.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
	}
	if showPrice {
		out.Price = s.Price
	}

	return out
}

func (b Build) stabsToRepo(s *api.BuildStabs) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func (b Build) switchEntriesToRepo(entries *[]api.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) keycapKitEntriesToRepo(entries *[]api.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

// keyboardRefToAPI resolves keyboardID into a denormalized reference plus
// its purchase price. Returns (nil, nil, nil) if the keyboard no longer
// exists.
//
// resolveImages false skips presigning ImageUrl. When true, the ref
// surfaces only the keyboard's first image (kb.Images[0]), mirroring how
// [Build.ToAPISummary] picks a build's own Images[0] for its summary
// thumbnail.
func (b Build) keyboardRefToAPI(
	ctx context.Context, ownerID, keyboardID string, resolveImages bool,
) (*api.BuildKeyboardRef, *float64, error) {
	kb, err := b.KeyboardRepo.Get(ctx, ownerID, keyboardID)
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

	if resolveImages {
		if imgs := repository.SortedKeyboardImages(kb.Images); len(imgs) > 0 {
			url, err := b.KeyboardImages.PresignGetKeyboardImage(ctx, imgs[0].Path)
			if err != nil {
				return nil, nil, fmt.Errorf("presigning keyboard image for keyboard %q: %w", keyboardID, err)
			}
			ref.ImageUrl = &url
		}
	}

	return ref, kb.Purchase.Price, nil
}

// switchEntriesResolvedToAPI resolves each entry's Switch id into a
// denormalized reference via a per-entry SwitchRepo.Get call - same
// per-item-fetch approach as [Build.ToAPISummary]'s keyboard lookup, run
// concurrently across entries since each only touches its own out[i]. An
// entry whose switch no longer exists keeps its Count but leaves Switch
// nil rather than dropping the entry or failing the request. Also returns
// the summed cost across entries with a known per-unit price: switches are
// bought in bulk (SwitchPurchase.Price is the total for Quantity units, not
// a per-unit price - see SwitchPurchase's doc), so an entry contributes
// (Price/Quantity)*Count only when Quantity is set and non-zero; otherwise
// its cost is unknown and excluded rather than guessed at.
//
// resolveImages false skips presigning ImageUrl.
func (b Build) switchEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildSwitchEntry, resolveImages bool,
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

			sw, err := b.SwitchRepo.Get(ctx, ownerID, e.Switch)
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
				url, err := b.SwitchImages.PresignGet(ctx, *sw.ImagePath)
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

// keycapKitEntriesResolvedToAPI resolves each entry's (KeycapSet, Kit) pair
// into a denormalized reference plus the kit's own name/image, via a
// per-entry KeycapSetRepo.Get call - same per-item-fetch approach as
// [Build.switchEntriesResolvedToAPI], run concurrently across entries for
// the same reason. An entry whose keycap set - or whose kit within it - no
// longer exists keeps its KitId but leaves KeycapSet, KitName, and
// KitImageUrl nil rather than dropping the entry or failing the request.
// Also returns the summed price across entries whose kit has a known
// price.
//
// resolveImages false skips presigning KitImageUrl.
func (b Build) keycapKitEntriesResolvedToAPI(
	ctx context.Context, ownerID string, entries []repository.BuildKeycapKitEntry, resolveImages bool,
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

			ks, err := b.KeycapSetRepo.Get(ctx, ownerID, e.KeycapSet)
			if err != nil {
				if !errors.Is(err, repository.ErrNotFound) {
					errs[i] = fmt.Errorf("getting keycap set %q: %w", e.KeycapSet, err)
				}
				return
			}

			kit := b.findKeycapKit(ks.Kits, e.Kit)
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
				url, err := b.KitImages.PresignGet(ctx, *kit.ImagePath)
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

func (b Build) findKeycapKit(kits map[string]repository.KeycapKit, kitID string) *repository.KeycapKit {
	kit, ok := kits[kitID]
	if !ok {
		return nil
	}
	return &kit
}

// imagesToAPI mints a fresh presigned GET URL per image, per request -
// never persisted, mirroring [KeycapSet.KitToAPI]'s handling of a kit's
// image. images is already ordered (by Seq) by the caller.
func (b Build) imagesToAPI(ctx context.Context, images []repository.BuildImage) (*[]api.BuildImage, error) {
	if len(images) == 0 {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.BuildImage, len(images))
	errs := make([]error, len(images))

	var wg sync.WaitGroup
	for i, img := range images {
		wg.Add(1)
		go func(i int, img repository.BuildImage) {
			defer wg.Done()

			url, err := b.Images.PresignGetBuildImage(ctx, img.Path)
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
