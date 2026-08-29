package repoapi

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToAPI maps a repository.KeycapSet to its wire representation.
// Returns an error if a stored kit's Purchase date doesn't match dateLayout,
// or if a kit has an ImagePath and images.PresignGet fails. Kits are mapped
// concurrently, sorted by kit_id for a stable order - each only touches its
// own slot in mapped, and a set can have an unbounded number of kits, each
// potentially needing its own S3 presign.
func KeycapSetToAPI(ctx context.Context, ks repository.KeycapSet, images repository.KeycapKitImageStore, isOwner bool) (api.KeycapSet, error) {
	var kits *[]api.KeycapKit
	if len(ks.Kits) > 0 {
		ids := sortedKitIDs(ks.Kits)
		mapped := make([]api.KeycapKit, len(ids))
		errs := make([]error, len(ids))

		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			go func(i int, k repository.KeycapKit) {
				defer wg.Done()

				apiKit, err := KeycapKitToAPI(ctx, k, images, isOwner)
				if err != nil {
					errs[i] = err
					return
				}
				mapped[i] = apiKit
			}(i, ks.Kits[id])
		}
		wg.Wait()

		if err := errors.Join(errs...); err != nil {
			return api.KeycapSet{}, err
		}

		kits = &mapped
	}

	return api.KeycapSet{
		Id:           ks.ID,
		Brand:        ks.Brand,
		Name:         ks.Name,
		Profile:      ks.Profile,
		Material:     ks.Material,
		Notes:        ks.Notes,
		Visibility:   api.Visibility(ks.Visibility),
		Kits:         kits,
		PrimaryKitId: validPrimaryKitID(ks.PrimaryKitID, ks.Kits),
		OrderStatus:  repository.AggregateOrderStatus(ks.Kits),
	}, nil
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

// KeycapSetToRepo maps a generated KeycapSetInput (already schema-validated
// by the OpenAPI request validator) to a repository.KeycapSet. It does not
// set UserID or ID - those come from the request's path/caller, not the
// body, and stay the handler's responsibility.
func KeycapSetToRepo(in api.KeycapSetInput) repository.KeycapSet {
	return repository.KeycapSet{
		Brand:      in.Brand,
		Name:       in.Name,
		Profile:    in.Profile,
		Material:   in.Material,
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// KeycapSetToAPISummary maps a repository.KeycapSet to the
// KeycapSetSummary schema returned by the list endpoint. PrimaryKitImage
// is nil unless PrimaryKitID names a kit still present in Kits and that
// kit has an ImagePath set, in which case it's a freshly minted presigned
// GET URL - never persisted, never cached, mirroring KeycapKitToAPI.
func KeycapSetToAPISummary(ctx context.Context, ks repository.KeycapSet, images repository.KeycapKitImageStore) (api.KeycapSetSummary, error) {
	summary := api.KeycapSetSummary{
		Id:          &ks.ID,
		Brand:       &ks.Brand,
		Name:        &ks.Name,
		Profile:     ks.Profile,
		OrderStatus: repository.AggregateOrderStatus(ks.Kits),
	}

	primaryKit := findKit(validPrimaryKitID(ks.PrimaryKitID, ks.Kits), ks.Kits)
	if primaryKit != nil && primaryKit.ImagePath != nil {
		url, err := images.PresignGet(ctx, *primaryKit.ImagePath)
		if err != nil {
			return api.KeycapSetSummary{}, fmt.Errorf("presigning primary kit image: %w", err)
		}
		summary.PrimaryKitImage = &api.KeycapKitImage{Url: url}
	}

	return summary, nil
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

// KeycapKitToAPI maps a repository.KeycapKit to its wire representation.
// Image is nil unless k.ImagePath is set, in which case it's a freshly
// minted presigned GET URL - never persisted, never cached.
func KeycapKitToAPI(ctx context.Context, k repository.KeycapKit, images repository.KeycapKitImageStore, isOwner bool) (api.KeycapKit, error) {
	purchase, err := keycapKitPurchaseToAPI(k.Purchase, isOwner)
	if err != nil {
		return api.KeycapKit{}, err
	}

	var image *api.KeycapKitImage
	if k.ImagePath != nil {
		url, err := images.PresignGet(ctx, *k.ImagePath)
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

// KeycapKitToRepo maps a generated KeycapKitInput (already schema-validated
// by the OpenAPI request validator) to a repository.KeycapKit. It does not
// set KitID - that's the handler's responsibility (server-generated on
// create).
func KeycapKitToRepo(in api.KeycapKitInput) repository.KeycapKit {
	return repository.KeycapKit{
		Name:     in.Name,
		Purchase: keycapKitPurchaseToRepo(in.Purchase),
	}
}

func keycapKitPurchaseToAPI(p repository.KeycapKitPurchase, isOwner bool) (*api.Purchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
	}
	if isOwner {
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

func keycapKitPurchaseToRepo(p *api.Purchase) repository.KeycapKitPurchase {
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
