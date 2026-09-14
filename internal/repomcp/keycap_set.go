package repomcp

import (
	"slices"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeycapSetToMCP maps a repository.KeycapSet to its MCP tool shape. Unlike
// repoapi.KeycapSetToAPI, this never presigns a GET URL for a kit's image -
// KeycapKitToMCP reports only HasImage, so mapping a set can't fail the way
// the REST mapping can on a presign error. The owner always sees each
// kit's own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers.
func KeycapSetToMCP(ks repository.KeycapSet, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeycapSet {
	showPrice := ownerPrefs.ShowPriceSingle(isOwner)
	var kits []schema.KeycapKit
	if len(ks.Kits) > 0 {
		ids := sortedKitIDs(ks.Kits)
		kits = make([]schema.KeycapKit, len(ids))
		for i, id := range ids {
			kits[i] = KeycapKitToMCP(ks.Kits[id], showPrice)
		}
	}

	return schema.KeycapSet{
		ID:           ks.ID,
		Brand:        ks.Brand,
		Name:         ks.Name,
		Profile:      ks.Profile,
		Material:     ks.Material,
		Notes:        ks.Notes,
		Visibility:   string(ks.Visibility),
		Kits:         kits,
		PrimaryKitID: validPrimaryKitID(ks.PrimaryKitID, ks.Kits),
		OrderStatus:  repository.AggregateOrderStatus(ks.Kits),
	}
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

// KeycapSetToMCPSummary lifts nothing extra out of the set beyond the
// summary fields themselves and PrimaryKitHasImage, which - like
// KeycapKitToMCP's HasImage - reports presence only, never a presigned
// URL a list result would then be stuck carrying a short-lived value in.
// TotalCost is shown per ownerPrefs.ShowPriceToMe (owner) or
// ownerPrefs.ShowPriceToOthers (non-owner) - unlike [KeycapSetToMCP], the
// owner isn't unconditionally shown price here.
func KeycapSetToMCPSummary(ks repository.KeycapSet, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.KeycapSetSummary {
	primaryKitID := validPrimaryKitID(ks.PrimaryKitID, ks.Kits)
	primaryKit := findKit(primaryKitID, ks.Kits)

	summary := schema.KeycapSetSummary{
		ID:                 ks.ID,
		Brand:              ks.Brand,
		Name:               ks.Name,
		Profile:            ks.Profile,
		PrimaryKitID:       primaryKitID,
		PrimaryKitHasImage: primaryKit != nil && primaryKit.ImagePath != nil,
		OrderStatus:        repository.AggregateOrderStatus(ks.Kits),
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		prices := make([]*float64, 0, len(ks.Kits))
		for _, k := range ks.Kits {
			prices = append(prices, k.Purchase.Price)
		}
		summary.TotalCost = sumKnownCosts(prices...)
	}

	return summary
}

// sumKnownCosts mirrors repoapi's helper of the same name: sums the
// non-nil components, treating an all-nil input as "no known cost"
// rather than zero.
func sumKnownCosts(components ...*float64) *float64 {
	var total float64
	var haveAny bool
	for _, c := range components {
		if c == nil {
			continue
		}
		total += *c
		haveAny = true
	}

	if !haveAny {
		return nil //nolint:nilnil // no known-priced components is a valid, expected result
	}

	return &total
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
// schema.KeycapKit for why. showPrice gates purchase.price - callers
// resolve it from isOwner/ownerPrefs themselves, since the right rule
// differs between the full-set GET (KeycapSetToMCP, owner unconditional)
// and standalone kit create/update (always the caller's own kit, so
// always true).
func KeycapKitToMCP(k repository.KeycapKit, showPrice bool) schema.KeycapKit {
	return schema.KeycapKit{
		KitID:    k.KitID,
		Name:     k.Name,
		HasImage: k.ImagePath != nil,
		Purchase: keycapKitPurchaseToMCP(k.Purchase, showPrice),
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
func keycapKitPurchaseToMCP(p repository.KeycapKitPurchase, showPrice bool) *schema.KeycapKitPurchase {
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
	if showPrice {
		out.Price = p.Price
	}

	return out
}
