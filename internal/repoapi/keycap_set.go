package repoapi

import (
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToAPI maps a repository.KeycapSet to its wire representation.
// Returns an error if a stored kit's Purchase date is malformed - the
// layout is enforced on write, but a row can still be poisoned by
// something outside that path (e.g. a manual DynamoDB edit or a restored
// backup), same rationale as KeyboardToAPI.
func KeycapSetToAPI(ks repository.KeycapSet) (api.KeycapSet, error) {
	var kits *[]api.KeycapKit
	if ks.Kits != nil {
		mapped := make([]api.KeycapKit, len(ks.Kits))
		for i, k := range ks.Kits {
			apiKit, err := KeycapKitToAPI(k)
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
// Image is always nil here - it's populated by a presigned URL only when
// the kit-image routes land (SetKeycapKitImage/DeleteKeycapKitImage), not
// by this mapper.
func KeycapKitToAPI(k repository.KeycapKit) (api.KeycapKit, error) {
	purchase, err := keycapKitPurchaseToAPI(k.Purchase)
	if err != nil {
		return api.KeycapKit{}, err
	}

	return api.KeycapKit{
		KitId:    k.KitID,
		Name:     k.Name,
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
