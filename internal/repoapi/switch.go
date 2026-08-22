package repoapi

import (
	"context"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// SwitchToAPI maps a repository.Switch to its wire representation. Returns
// an error if a stored Purchase date doesn't match dateLayout, or an image
// fails to presign.
func SwitchToAPI(ctx context.Context, sw repository.Switch, images repository.SwitchImageStore, isOwner bool) (api.Switch, error) {
	purchase, err := switchPurchaseToAPI(sw.Purchase, isOwner)
	if err != nil {
		return api.Switch{}, err
	}

	var image *api.SwitchImage
	if sw.ImagePath != nil {
		url, err := images.PresignGet(ctx, *sw.ImagePath)
		if err != nil {
			return api.Switch{}, fmt.Errorf("presigning switch image: %w", err)
		}
		image = &api.SwitchImage{Url: url}
	}

	return api.Switch{
		Id:           sw.ID,
		Brand:        sw.Brand,
		Manufacturer: sw.Manufacturer,
		Name:         sw.Name,
		Type:         sw.Type,
		Pins:         sw.Pins,
		FactoryLubed: sw.FactoryLubed,
		Material:     switchMaterialToAPI(sw.Material),
		Force:        switchForceToAPI(sw.Force),
		Spring:       switchSpringToAPI(sw.Spring),
		Purchase:     purchase,
		Notes:        sw.Notes,
		Visibility:   api.Visibility(sw.Visibility),
		Image:        image,
	}, nil
}

// SwitchToRepo maps a generated SwitchInput (already schema-validated by the
// OpenAPI request validator) to a repository.Switch. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility.
func SwitchToRepo(in api.SwitchInput) repository.Switch {
	return repository.Switch{
		Brand:        in.Brand,
		Manufacturer: in.Manufacturer,
		Name:         in.Name,
		Type:         in.Type,
		Pins:         in.Pins,
		FactoryLubed: in.FactoryLubed,
		Material:     switchMaterialToRepo(in.Material),
		Force:        switchForceToRepo(in.Force),
		Spring:       switchSpringToRepo(in.Spring),
		Purchase:     switchPurchaseToRepo(in.Purchase),
		Notes:        in.Notes,
		Visibility:   repository.Visibility(in.Visibility),
	}
}

// SwitchToAPISummary maps a repository.Switch to the SwitchSummary schema
// returned by the list endpoint, presigning its image if it has one.
func SwitchToAPISummary(ctx context.Context, sw repository.Switch, images repository.SwitchImageStore) (api.SwitchSummary, error) {
	summary := api.SwitchSummary{
		Id:    &sw.ID,
		Brand: &sw.Brand,
		Name:  &sw.Name,
		Type:  &sw.Type,
	}

	if sw.ImagePath != nil {
		url, err := images.PresignGet(ctx, *sw.ImagePath)
		if err != nil {
			return api.SwitchSummary{}, fmt.Errorf("presigning switch image: %w", err)
		}
		summary.Image = &api.SwitchImage{Url: url}
	}

	return summary, nil
}

func switchMaterialToAPI(m repository.SwitchMaterial) *api.SwitchMaterial {
	if m.TopHousing == nil && m.BottomHousing == nil && m.Stem == nil {
		return nil
	}

	return &api.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func switchMaterialToRepo(m *api.SwitchMaterial) repository.SwitchMaterial {
	if m == nil {
		return repository.SwitchMaterial{}
	}

	return repository.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func switchForceToAPI(f repository.SwitchForce) *api.SwitchForce {
	if f.Actuation == nil && f.BottomOut == nil {
		return nil
	}

	return &api.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func switchForceToRepo(f *api.SwitchForce) repository.SwitchForce {
	if f == nil {
		return repository.SwitchForce{}
	}

	return repository.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func switchSpringToAPI(s repository.SwitchSpring) *api.SwitchSpring {
	if s.Material == nil && s.PreTravel == nil && s.TotalTravel == nil {
		return nil
	}

	return &api.SwitchSpring{
		Material:    s.Material,
		PreTravel:   s.PreTravel,
		TotalTravel: s.TotalTravel,
	}
}

func switchSpringToRepo(s *api.SwitchSpring) repository.SwitchSpring {
	if s == nil {
		return repository.SwitchSpring{}
	}

	return repository.SwitchSpring{
		Material:    s.Material,
		PreTravel:   s.PreTravel,
		TotalTravel: s.TotalTravel,
	}
}

func switchPurchaseToAPI(p repository.SwitchPurchase, isOwner bool) (*api.SwitchPurchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil &&
		p.OrderStatus == nil && p.Quantity == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.SwitchPurchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
		Quantity:    p.Quantity,
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

func switchPurchaseToRepo(p *api.SwitchPurchase) repository.SwitchPurchase {
	if p == nil {
		return repository.SwitchPurchase{}
	}

	out := repository.SwitchPurchase{
		Vendor:      p.Vendor,
		Price:       p.Price,
		OrderStatus: p.OrderStatus,
		Quantity:    p.Quantity,
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
