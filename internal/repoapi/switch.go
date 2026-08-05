package repoapi

import (
	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// SwitchToAPI maps a repository.Switch to its wire representation.
func SwitchToAPI(sw repository.Switch) api.Switch {
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
		Purchase:     switchPurchaseToAPI(sw.Purchase),
		Notes:        sw.Notes,
		Visibility:   api.Visibility(sw.Visibility),
	}
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
// returned by the list endpoint.
func SwitchToAPISummary(sw repository.Switch) api.SwitchSummary {
	return api.SwitchSummary{
		Id:    &sw.ID,
		Brand: &sw.Brand,
		Name:  &sw.Name,
		Type:  &sw.Type,
	}
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

func switchPurchaseToAPI(p repository.SwitchPurchase) *api.SwitchPurchase {
	if p.Vendor == nil && p.Price == nil && p.Quantity == nil {
		return nil
	}

	return &api.SwitchPurchase{
		Vendor:   p.Vendor,
		Price:    p.Price,
		Quantity: p.Quantity,
	}
}

func switchPurchaseToRepo(p *api.SwitchPurchase) repository.SwitchPurchase {
	if p == nil {
		return repository.SwitchPurchase{}
	}

	return repository.SwitchPurchase{
		Vendor:   p.Vendor,
		Price:    p.Price,
		Quantity: p.Quantity,
	}
}
