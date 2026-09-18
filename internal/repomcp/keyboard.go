package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// Keyboard maps repository.Keyboard to and from its MCP tool shape. It has
// no dependencies - unlike repoapi.Keyboard, this never presigns image
// URLs; ToMCP reports only HasImages.
type Keyboard struct{}

// ToMCP maps a repository.Keyboard to its MCP tool shape. Pointers pass
// through undereferenced so a recorded zero survives, as
// [repoapi.Keyboard.ToAPI] does. The owner always sees their own
// purchase.price; a non-owner sees it only if ownerPrefs.ShowPriceToOthers.
func (k Keyboard) ToMCP(kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.Keyboard {
	return schema.Keyboard{
		ID:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     k.designToMCP(kb.Design),
		PCB:        k.pcbToMCP(kb.PCB),
		Purchase:   k.purchaseToMCP(kb.Purchase, ownerPrefs.ShowPriceSingle(isOwner)),
		Notes:      kb.Notes,
		Visibility: string(kb.Visibility),
		HasImages:  len(kb.Images) > 0,
	}
}

// ToMCPSummary lifts order_status out of purchase, so a keyboard still on
// order is visible while browsing a list. Price is shown per
// ownerPrefs.ShowPriceToMe (owner) or ownerPrefs.ShowPriceToOthers
// (non-owner) - unlike [Keyboard.ToMCP], the owner isn't unconditionally
// shown price here.
func (k Keyboard) ToMCPSummary(kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeyboardSummary {
	summary := schema.KeyboardSummary{
		ID:          kb.ID,
		Brand:       kb.Brand,
		Name:        kb.Name,
		Size:        kb.Size,
		Layout:      kb.Layout,
		OrderStatus: kb.Purchase.OrderStatus,
		HasImages:   len(kb.Images) > 0,
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		summary.Price = kb.Purchase.Price
	}

	return summary
}

func (k Keyboard) designToMCP(d repository.KeyboardDesign) *schema.KeyboardDesign {
	topCase := k.materialColorToMCP(d.TopCase)
	bottomCase := k.materialColorToMCP(d.BottomCase)
	weight := k.materialColorToMCP(d.Weight)

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

func (k Keyboard) materialColorToMCP(mc repository.KeyboardMaterialColor) *schema.KeyboardMaterialColor {
	if mc.Material == nil && mc.Color == nil {
		return nil
	}

	return &schema.KeyboardMaterialColor{
		Material: mc.Material,
		Color:    mc.Color,
	}
}

func (k Keyboard) pcbToMCP(p repository.KeyboardPCB) *schema.KeyboardPCB {
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

// Dates pass through as strings, unlike [repoapi.Keyboard.ToAPI], so this
// can't fail on a malformed one.
func (k Keyboard) purchaseToMCP(p repository.KeyboardPurchase, showPrice bool) *schema.KeyboardPurchase {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil &&
		p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil
	}

	out := &schema.KeyboardPurchase{
		Vendor:       p.Vendor,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
	if showPrice {
		out.Price = p.Price
	}

	return out
}

// FromMCP maps a create_keyboard/update_keyboard tool argument to its
// repository shape. ID and UserID are left unset: the caller sets ID, and
// UserID comes from ctx in the repository layer.
func (k Keyboard) FromMCP(in schema.KeyboardInput) repository.Keyboard {
	return repository.Keyboard{
		Brand:      in.Brand,
		Name:       in.Name,
		Size:       in.Size,
		Layout:     in.Layout,
		Design:     k.designFromMCP(in.Design),
		PCB:        k.pcbFromMCP(in.PCB),
		Purchase:   k.purchaseFromMCP(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

func (k Keyboard) designFromMCP(d *schema.KeyboardDesign) repository.KeyboardDesign {
	if d == nil {
		return repository.KeyboardDesign{}
	}

	return repository.KeyboardDesign{
		TopCase:    k.materialColorFromMCP(d.TopCase),
		BottomCase: k.materialColorFromMCP(d.BottomCase),
		Weight:     k.materialColorFromMCP(d.Weight),
		Plates:     d.Plates,
	}
}

func (k Keyboard) materialColorFromMCP(mc *schema.KeyboardMaterialColor) repository.KeyboardMaterialColor {
	if mc == nil {
		return repository.KeyboardMaterialColor{}
	}

	return repository.KeyboardMaterialColor{
		Material: mc.Material,
		Color:    mc.Color,
	}
}

func (k Keyboard) pcbFromMCP(p *schema.KeyboardPCB) repository.KeyboardPCB {
	if p == nil {
		return repository.KeyboardPCB{}
	}

	return repository.KeyboardPCB{
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
	}
}

func (k Keyboard) purchaseFromMCP(p *schema.KeyboardPurchase) repository.KeyboardPurchase {
	if p == nil {
		return repository.KeyboardPurchase{}
	}

	return repository.KeyboardPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
}
