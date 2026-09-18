package repoapi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSet maps repository.KeycapSet to and from its wire representations.
type KeycapSet struct {
	Images     repository.KeycapKitImageStore
	Repo       repository.KeycapSetRepository
	PresignTTL time.Duration
}

// ToAPI maps a repository.KeycapSet to its wire representation. The owner
// always sees each kit's own Purchase.Price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers. Returns an error if a stored kit's Purchase
// date doesn't match dateLayout, or if a kit has an ImagePath and
// Images.PresignGet fails. Kits are mapped concurrently, sorted by kit_id
// for a stable order - each only touches its own slot in mapped, and a set
// can have an unbounded number of kits, each potentially needing its own S3
// presign.
func (ks KeycapSet) ToAPI(ctx context.Context, set repository.KeycapSet, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeycapSet, error) {
	showPrice := ownerPrefs.ShowPriceSingle(isOwner)
	var kits *[]api.KeycapKit
	if len(set.Kits) > 0 {
		ids := sortedKitIDs(set.Kits)
		mapped := make([]api.KeycapKit, len(ids))
		errs := make([]error, len(ids))

		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func(i int, k repository.KeycapKit) {
				defer wg.Done()

				apiKit, err := ks.KitToAPI(ctx, set.UserID, set.ID, k, showPrice)
				if err != nil {
					errs[i] = err
					return
				}
				mapped[i] = apiKit
			}(i, set.Kits[id])
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			return api.KeycapSet{}, err
		}

		kits = &mapped
	}

	return api.KeycapSet{
		Id:           set.ID,
		Brand:        set.Brand,
		Name:         set.Name,
		Profile:      set.Profile,
		Material:     set.Material,
		Notes:        set.Notes,
		Visibility:   api.Visibility(set.Visibility),
		Kits:         kits,
		PrimaryKitId: validPrimaryKitID(set.PrimaryKitID, set.Kits),
		OrderStatus:  repository.AggregateOrderStatus(set.Kits),
	}, nil
}

// ToRepo maps a generated KeycapSetInput (already schema-validated by the
// OpenAPI request validator) to a repository.KeycapSet. It does not set
// UserID or ID - those come from the request's path/caller, not the body,
// and stay the handler's responsibility.
func (ks KeycapSet) ToRepo(in api.KeycapSetInput) repository.KeycapSet {
	return repository.KeycapSet{
		Brand:      in.Brand,
		Name:       in.Name,
		Profile:    in.Profile,
		Material:   in.Material,
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// ToAPISummary maps a repository.KeycapSet to the KeycapSetSummary schema
// returned by the list endpoint. PrimaryKitImage is nil unless
// PrimaryKitID names a kit still present in Kits and that kit has an
// ImagePath set, in which case it's a freshly minted presigned GET URL -
// never persisted, never cached, mirroring [KeycapSet.KitToAPI]. TotalCost
// is shown per ownerPrefs.ShowPriceToMe (owner) or
// ownerPrefs.ShowPriceToOthers (non-owner) - unlike [KeycapSet.ToAPI], the
// owner isn't unconditionally shown price here.
func (ks KeycapSet) ToAPISummary(ctx context.Context, set repository.KeycapSet, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeycapSetSummary, error) {
	summary := api.KeycapSetSummary{
		Id:          &set.ID,
		Brand:       &set.Brand,
		Name:        &set.Name,
		Profile:     set.Profile,
		OrderStatus: repository.AggregateOrderStatus(set.Kits),
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		prices := make([]*float64, 0, len(set.Kits))
		for _, k := range set.Kits {
			prices = append(prices, k.Purchase.Price)
		}
		summary.TotalCost = sumKnownCosts(prices...)
	}

	primaryKit := findKit(validPrimaryKitID(set.PrimaryKitID, set.Kits), set.Kits)
	if primaryKit != nil && primaryKit.ImagePath != nil {
		url, err := ks.resolveKeycapKitImageURL(ctx, set.UserID, set.ID, *primaryKit)
		if err != nil {
			return api.KeycapSetSummary{}, fmt.Errorf("presigning primary kit image: %w", err)
		}
		summary.PrimaryKitImage = &api.KeycapKitImage{Url: url}
	}

	return summary, nil
}

// KitToAPI maps a repository.KeycapKit to its wire representation. Image
// is nil unless k.ImagePath is set, in which case it's a presigned GET URL,
// reused from cache if still fresh. showPrice gates Purchase.Price -
// callers resolve it from isOwner/ownerPrefs themselves, since the right
// rule differs between the full-set GET ([KeycapSet.ToAPI], owner
// unconditional) and standalone kit create/update (always the caller's own
// kit, so always true).
func (ks KeycapSet) KitToAPI(ctx context.Context, ownerID, setID string, k repository.KeycapKit, showPrice bool) (api.KeycapKit, error) {
	purchase, err := ks.kitPurchaseToAPI(k.Purchase, showPrice)
	if err != nil {
		return api.KeycapKit{}, err
	}

	var image *api.KeycapKitImage
	if k.ImagePath != nil {
		url, err := ks.resolveKeycapKitImageURL(ctx, ownerID, setID, k)
		if err != nil {
			return api.KeycapKit{}, fmt.Errorf("presigning kit image: %w", err)
		}
		image = &api.KeycapKitImage{Url: url}
	}

	return api.KeycapKit{
		KitId:    k.KitID,
		Name:     k.Name,
		Image:    image,
		Purchase: purchase,
	}, nil
}

// resolveKeycapKitImageURL presigns k.ImagePath, reusing its cached GET URL
// if still fresh enough. Callers must check k.ImagePath != nil first.
func (ks KeycapSet) resolveKeycapKitImageURL(ctx context.Context, ownerID, setID string, k repository.KeycapKit) (string, error) {
	path := *k.ImagePath

	return resolveImageURL(k.GetURL, k.GetURLExpiresAt, ks.PresignTTL,
		func() (string, error) { return ks.Images.PresignGet(ctx, path) },
		func(url string, expiresAt time.Time) error {
			_, err := ks.Repo.SetKitImageGetCache(ctx, ownerID, setID, k.KitID, path, url, expiresAt)
			return err
		},
	)
}

// KitToRepo maps a generated KeycapKitInput (already schema-validated by
// the OpenAPI request validator) to a repository.KeycapKit. It does not
// set KitID - that's the handler's responsibility (server-generated on
// create).
func (ks KeycapSet) KitToRepo(in api.KeycapKitInput) repository.KeycapKit {
	return repository.KeycapKit{
		Name:     in.Name,
		Purchase: ks.kitPurchaseToRepo(in.Purchase),
	}
}

func (ks KeycapSet) kitPurchaseToAPI(p repository.KeycapKitPurchase, showPrice bool) (*api.Purchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
	}
	if showPrice {
		out.Price = p.Price
	}
	if p.OrderDate != nil {
		d, err := parseAPIDate(*p.OrderDate)
		if err != nil {
			return nil, fmt.Errorf("parsing order_date: %w", err)
		}
		out.OrderDate = d
	}
	if p.DeliveryDate != nil {
		d, err := parseAPIDate(*p.DeliveryDate)
		if err != nil {
			return nil, fmt.Errorf("parsing delivery_date: %w", err)
		}
		out.DeliveryDate = d
	}

	return out, nil
}

func (ks KeycapSet) kitPurchaseToRepo(p *api.Purchase) repository.KeycapKitPurchase {
	if p == nil {
		return repository.KeycapKitPurchase{}
	}

	out := repository.KeycapKitPurchase{
		Vendor:      p.Vendor,
		Price:       p.Price,
		OrderStatus: p.OrderStatus,
	}
	if p.OrderDate != nil {
		s := p.OrderDate.Format(dateLayout)
		out.OrderDate = &s
	}
	if p.DeliveryDate != nil {
		s := p.DeliveryDate.Format(dateLayout)
		out.DeliveryDate = &s
	}

	return out
}

// validPrimaryKitID returns primaryKitID unchanged if it names a kit still
// present in kits, or nil otherwise (never set, or naming a since-deleted
// kit) - callers must not surface a dangling reference.
func validPrimaryKitID(primaryKitID *string, kits map[string]repository.KeycapKit) *string {
	if primaryKitID == nil {
		return nil
	}
	if _, ok := kits[*primaryKitID]; !ok {
		return nil
	}
	return primaryKitID
}

// sortedKitIDs returns kits' keys sorted, for a deterministic output order.
func sortedKitIDs(kits map[string]repository.KeycapKit) []string {
	ids := make([]string, 0, len(kits))
	for id := range kits {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// findKit returns the kit in kits with the given kitID, or nil if kitID
// is nil or names no kit in kits.
func findKit(kitID *string, kits map[string]repository.KeycapKit) *repository.KeycapKit {
	if kitID == nil {
		return nil
	}
	kit, ok := kits[*kitID]
	if !ok {
		return nil
	}
	return &kit
}
