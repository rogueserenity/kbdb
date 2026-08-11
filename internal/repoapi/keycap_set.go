package repoapi

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToAPI maps a repository.KeycapSet to its wire representation.
// Returns an error if a stored kit's Purchase date doesn't match dateLayout,
// or if a kit has an ImagePath and images.PresignGet fails.
func KeycapSetToAPI(ctx context.Context, ks repository.KeycapSet, images repository.KeycapKitImageStore) (api.KeycapSet, error) {
	var kits *[]api.KeycapKit
	if ks.Kits != nil {
		mapped := make([]api.KeycapKit, len(ks.Kits))
		for i, k := range ks.Kits {
			apiKit, err := KeycapKitToAPI(ctx, k, images)
			if err != nil {
				return api.KeycapSet{}, err
			}
			mapped[i] = apiKit
		}
		kits = &mapped
	}

	return api.KeycapSet{
		Id:         ks.ID,
		Brand:      ks.Brand,
		Name:       ks.Name,
		Profile:    ks.Profile,
		Material:   ks.Material,
		Notes:      ks.Notes,
		Visibility: api.Visibility(ks.Visibility),
		Kits:       kits,
	}, nil
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
// KeycapSetSummary schema returned by the list endpoint.
func KeycapSetToAPISummary(ks repository.KeycapSet) api.KeycapSetSummary {
	return api.KeycapSetSummary{
		Id:      &ks.ID,
		Brand:   &ks.Brand,
		Name:    &ks.Name,
		Profile: ks.Profile,
	}
}

// KeycapKitToAPI maps a repository.KeycapKit to its wire representation.
// Image is nil unless k.ImagePath is set, in which case it's a freshly
// minted presigned GET URL - never persisted, never cached.
func KeycapKitToAPI(ctx context.Context, k repository.KeycapKit, images repository.KeycapKitImageStore) (api.KeycapKit, error) {
	purchase, err := keycapKitPurchaseToAPI(k.Purchase)
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

func keycapKitPurchaseToAPI(p repository.KeycapKitPurchase) (*api.Purchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		Price:       p.Price,
		OrderStatus: p.OrderStatus,
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
