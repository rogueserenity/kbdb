package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToMCP maps a repository.KeycapSet to its MCP tool shape. Unlike
// repoapi.KeycapSetToAPI, this never presigns a GET URL for a kit's image -
// KeycapKitToMCP reports only HasImage, so mapping a set can't fail the way
// the REST mapping can on a presign error. isOwner gates each kit's
// purchase.price the same way repoapi.KeycapSetToAPI's does.
func KeycapSetToMCP(ks repository.KeycapSet, isOwner bool) schema.KeycapSet {
	var kits []schema.KeycapKit
	if ks.Kits != nil {
		kits = make([]schema.KeycapKit, len(ks.Kits))
		for i, k := range ks.Kits {
			kits[i] = KeycapKitToMCP(k, isOwner)
		}
	}

	return schema.KeycapSet{
		ID:         ks.ID,
		Brand:      ks.Brand,
		Name:       ks.Name,
		Profile:    ks.Profile,
		Material:   ks.Material,
		Notes:      ks.Notes,
		Visibility: string(ks.Visibility),
		Kits:       kits,
	}
}

// KeycapSetToMCPSummary lifts nothing extra out of the set - unlike
// KeyboardToMCPSummary's order_status, a keycap set's own order status lives
// per-kit, not on the set, so there's nothing meaningful to surface while
// browsing a list beyond the summary fields themselves.
func KeycapSetToMCPSummary(ks repository.KeycapSet) schema.KeycapSetSummary {
	return schema.KeycapSetSummary{
		ID:      ks.ID,
		Brand:   ks.Brand,
		Name:    ks.Name,
		Profile: ks.Profile,
	}
}

// KeycapSetFromMCP maps a create_keycap_set/update_keycap_set tool argument
// to its repository shape. ID and UserID are left unset: the caller sets
// ID, and UserID comes from ctx in the repository layer. Kits are left
// unset too - a set write never carries kits, which are managed one at a
// time via their own tools.
func KeycapSetFromMCP(in schema.KeycapSetInput) repository.KeycapSet {
	return repository.KeycapSet{
		Brand:      in.Brand,
		Name:       in.Name,
		Profile:    in.Profile,
		Material:   in.Material,
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// KeycapKitToMCP maps a repository.KeycapKit to its MCP tool shape.
// ImagePath collapses to the HasImage bool, never a URL - see
// schema.KeycapKit for why. isOwner gates purchase.price the same way
// repoapi.KeycapKitToAPI's does.
func KeycapKitToMCP(k repository.KeycapKit, isOwner bool) schema.KeycapKit {
	return schema.KeycapKit{
		KitID:    k.KitID,
		Name:     k.Name,
		HasImage: k.ImagePath != nil,
		Purchase: keycapKitPurchaseToMCP(k.Purchase, isOwner),
	}
}

// KeycapKitFromMCP maps a create_keycap_kit/update_keycap_kit tool argument
// to its repository shape. KitID and ImagePath are left unset: the caller
// sets KitID (fresh on create, preserved on update), and a kit's image is
// managed entirely through its own tools, never carried in a kit write.
func KeycapKitFromMCP(in schema.KeycapKitInput) repository.KeycapKit {
	return repository.KeycapKit{
		Name:     in.Name,
		Purchase: keycapKitPurchaseFromMCP(in.Purchase),
	}
}

func keycapKitPurchaseFromMCP(p *schema.KeycapKitPurchase) repository.KeycapKitPurchase {
	if p == nil {
		return repository.KeycapKitPurchase{}
	}

	return repository.KeycapKitPurchase{
		Vendor:       p.Vendor,
		Price:        p.Price,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
}

// Dates pass through as strings, unlike repoapi's mapping, so this can't
// fail on a malformed one. Mirrors [keyboardPurchaseToMCP].
func keycapKitPurchaseToMCP(p repository.KeycapKitPurchase, isOwner bool) *schema.KeycapKitPurchase {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil &&
		p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil
	}

	out := &schema.KeycapKitPurchase{
		Vendor:       p.Vendor,
		OrderDate:    p.OrderDate,
		DeliveryDate: p.DeliveryDate,
		OrderStatus:  p.OrderStatus,
	}
	if isOwner {
		out.Price = p.Price
	}

	return out
}
