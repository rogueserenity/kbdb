package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeyboardToMCP maps a repository.Keyboard to its MCP tool shape. Optional
// fields pass through as pointers rather than being dereferenced, so a
// recorded zero survives the round trip instead of being indistinguishable
// from unset - the same reason repoapi.KeyboardToAPI keeps them. The nested
// design/pcb/purchase groups collapse to nil when every field in them is
// unset, so an all-empty group is omitted entirely.
func KeyboardToMCP(kb repository.Keyboard) schema.Keyboard {
	return schema.Keyboard{
		ID:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     keyboardDesignToMCP(kb.Design),
		PCB:        keyboardPCBToMCP(kb.PCB),
		Purchase:   keyboardPurchaseToMCP(kb.Purchase),
		Notes:      kb.Notes,
		Visibility: string(kb.Visibility),
	}
}

// KeyboardToMCPSummary maps a repository.Keyboard to the abbreviated shape
// list_keyboards returns.
func KeyboardToMCPSummary(kb repository.Keyboard) schema.KeyboardSummary {
	return schema.KeyboardSummary{
		ID:          kb.ID,
		Brand:       kb.Brand,
		Name:        kb.Name,
		Size:        kb.Size,
		Layout:      kb.Layout,
		OrderStatus: kb.Purchase.OrderStatus,
	}
}

func keyboardDesignToMCP(d repository.KeyboardDesign) *schema.KeyboardDesign {
	topCase := keyboardMaterialColorToMCP(d.TopCase)
	bottomCase := keyboardMaterialColorToMCP(d.BottomCase)
	weight := keyboardMaterialColorToMCP(d.Weight)

	if topCase == nil && bottomCase == nil && weight == nil && len(d.Plates) == 0 {
		return nil
	}

	return &schema.KeyboardDesign{
		TopCase:    topCase,
		BottomCase: bottomCase,
		Weight:     weight,
		Plates:     d.Plates,
	}
}

func keyboardMaterialColorToMCP(mc repository.KeyboardMaterialColor) *schema.KeyboardMaterialColor {
	if mc.Material == nil && mc.Color == nil {
		return nil
	}

	return &schema.KeyboardMaterialColor{
		Material: mc.Material,
		Color:    mc.Color,
	}
}

func keyboardPCBToMCP(p repository.KeyboardPCB) *schema.KeyboardPCB {
	if p.Thickness == nil && p.Firmware == nil && p.Assembly == nil && p.Connectivity == nil {
		return nil
	}

	return &schema.KeyboardPCB{
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
	}
}

// Dates pass through as strings, so unlike repoapi.KeyboardToAPI - which
// parses them into openapi_types.Date and errors on a poisoned row - this
// can't fail. An agent reading a malformed stored date sees it verbatim
// rather than getting nothing at all.
func keyboardPurchaseToMCP(p repository.KeyboardPurchase) *schema.KeyboardPurchase {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil &&
		p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil
	}

	return &schema.KeyboardPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
}
