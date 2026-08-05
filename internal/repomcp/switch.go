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
func SwitchToMCP(sw repository.Switch) schema.Switch {
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
		Purchase:     switchPurchaseToMCP(sw.Purchase),
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

func switchPurchaseToMCP(p repository.SwitchPurchase) *schema.SwitchPurchase {
	if p.Vendor == nil && p.Price == nil && p.Quantity == nil {
		return nil
	}

	return &schema.SwitchPurchase{
		Vendor:   p.Vendor,
		Price:    p.Price,
		Quantity: p.Quantity,
	}
}
