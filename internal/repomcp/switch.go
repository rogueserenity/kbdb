package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// SwitchToMCP maps a repository.Switch to its MCP tool shape. Optional
// fields pass through as pointers rather than being dereferenced, so a
// recorded zero survives the round trip instead of being indistinguishable
// from unset - the same reason repoapi.SwitchToAPI keeps them. The nested
// material/force/spring/purchase groups still collapse to nil when every
// field in them is unset, so an all-empty group is omitted entirely.
// isOwner hides purchase.price from non-owners.
func SwitchToMCP(sw repository.Switch, isOwner bool) schema.Switch {
	return schema.Switch{
		ID:           sw.ID,
		Brand:        sw.Brand,
		Manufacturer: sw.Manufacturer,
		Name:         sw.Name,
		Type:         sw.Type,
		Pins:         sw.Pins,
		FactoryLubed: sw.FactoryLubed,
		Material:     switchMaterialToMCP(sw.Material),
		Force:        switchForceToMCP(sw.Force),
		Spring:       switchSpringToMCP(sw.Spring),
		Purchase:     switchPurchaseToMCP(sw.Purchase, isOwner),
		Notes:        sw.Notes,
		Visibility:   string(sw.Visibility),
	}
}

// SwitchToMCPSummary maps a repository.Switch to the abbreviated shape
// list_switches returns.
func SwitchToMCPSummary(sw repository.Switch) schema.SwitchSummary {
	return schema.SwitchSummary{
		ID:    sw.ID,
		Brand: sw.Brand,
		Name:  sw.Name,
		Type:  sw.Type,
	}
}

func switchMaterialToMCP(m repository.SwitchMaterial) *schema.SwitchMaterial {
	if m.TopHousing == nil && m.BottomHousing == nil && m.Stem == nil {
		return nil
	}

	return &schema.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func switchForceToMCP(f repository.SwitchForce) *schema.SwitchForce {
	if f.Actuation == nil && f.BottomOut == nil {
		return nil
	}

	return &schema.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func switchSpringToMCP(s repository.SwitchSpring) *schema.SwitchSpring {
	if s.Material == nil && s.PreTravel == nil && s.TotalTravel == nil {
		return nil
	}

	return &schema.SwitchSpring{
		Material:    s.Material,
		PreTravel:   s.PreTravel,
		TotalTravel: s.TotalTravel,
	}
}

// Dates pass through as strings, unlike repoapi.SwitchToAPI, so this can't
// fail on a malformed one.
func switchPurchaseToMCP(p repository.SwitchPurchase, isOwner bool) *schema.SwitchPurchase {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil &&
		p.DeliveryDate == nil && p.OrderStatus == nil && p.Quantity == nil {
		return nil
	}

	out := &schema.SwitchPurchase{
		Vendor:       p.Vendor,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
		Quantity:     p.Quantity,
	}
	if isOwner {
		out.Price = p.Price
	}

	return out
}

// SwitchFromMCP maps a create_switch/update_switch tool argument to its
// repository shape. ID and UserID are left unset: the caller sets ID (fresh
// for a create, the target's for an update), and UserID comes from ctx in
// the repository layer.
func SwitchFromMCP(in schema.SwitchInput) repository.Switch {
	return repository.Switch{
		Brand:        in.Brand,
		Manufacturer: in.Manufacturer,
		Name:         in.Name,
		Type:         in.Type,
		Pins:         in.Pins,
		FactoryLubed: in.FactoryLubed,
		Material:     switchMaterialFromMCP(in.Material),
		Force:        switchForceFromMCP(in.Force),
		Spring:       switchSpringFromMCP(in.Spring),
		Purchase:     switchPurchaseFromMCP(in.Purchase),
		Notes:        in.Notes,
		Visibility:   repository.Visibility(in.Visibility),
	}
}

func switchMaterialFromMCP(m *schema.SwitchMaterial) repository.SwitchMaterial {
	if m == nil {
		return repository.SwitchMaterial{}
	}

	return repository.SwitchMaterial{
		TopHousing:    m.TopHousing,
		BottomHousing: m.BottomHousing,
		Stem:          m.Stem,
	}
}

func switchForceFromMCP(f *schema.SwitchForce) repository.SwitchForce {
	if f == nil {
		return repository.SwitchForce{}
	}

	return repository.SwitchForce{
		Actuation: f.Actuation,
		BottomOut: f.BottomOut,
	}
}

func switchSpringFromMCP(s *schema.SwitchSpring) repository.SwitchSpring {
	if s == nil {
		return repository.SwitchSpring{}
	}

	return repository.SwitchSpring{
		Material:    s.Material,
		PreTravel:   s.PreTravel,
		TotalTravel: s.TotalTravel,
	}
}

func switchPurchaseFromMCP(p *schema.SwitchPurchase) repository.SwitchPurchase {
	if p == nil {
		return repository.SwitchPurchase{}
	}

	return repository.SwitchPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
		Quantity:     p.Quantity,
	}
}
