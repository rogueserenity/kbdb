package repoapi

import (
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// KeyboardToAPI maps a repository.Keyboard to its wire representation.
// isOwner gates purchase.price: a non-owner (including an anonymous caller)
// never sees the dollar amount, though the rest of purchase (vendor, dates,
// order_status) is unaffected - price is the only field callers agreed is
// sensitive. Returns an error if a stored Purchase date doesn't match
// dateLayout.
func KeyboardToAPI(kb repository.Keyboard, isOwner bool) (api.Keyboard, error) {
	purchase, err := keyboardPurchaseToAPI(kb.Purchase, isOwner)
	if err != nil {
		return api.Keyboard{}, err
	}

	return api.Keyboard{
		Id:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     keyboardDesignToAPI(kb.Design),
		Pcb:        keyboardPCBToAPI(kb.PCB),
		Purchase:   purchase,
		Notes:      kb.Notes,
		Visibility: api.Visibility(kb.Visibility),
	}, nil
}

// KeyboardToRepo maps a generated KeyboardInput (already schema-validated by
// the OpenAPI request validator) to a repository.Keyboard. It does not set
// UserID or ID - those come from the request's path/caller, not the body,
// and stay the handler's responsibility.
func KeyboardToRepo(in api.KeyboardInput) repository.Keyboard {
	return repository.Keyboard{
		Brand:      in.Brand,
		Name:       in.Name,
		Size:       in.Size,
		Layout:     in.Layout,
		Design:     keyboardDesignToRepo(in.Design),
		PCB:        keyboardPCBToRepo(in.Pcb),
		Purchase:   keyboardPurchaseToRepo(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// KeyboardToAPISummary maps a repository.Keyboard to the KeyboardSummary
// schema returned by the list endpoint.
func KeyboardToAPISummary(kb repository.Keyboard) api.KeyboardSummary {
	return api.KeyboardSummary{
		Id:          &kb.ID,
		Brand:       &kb.Brand,
		Name:        &kb.Name,
		Size:        kb.Size,
		Layout:      kb.Layout,
		OrderStatus: kb.Purchase.OrderStatus,
	}
}

func keyboardMaterialColorToAPI(m repository.KeyboardMaterialColor) *api.MaterialColor {
	if m.Material == nil && m.Color == nil {
		return nil
	}

	return &api.MaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func keyboardMaterialColorToRepo(m *api.MaterialColor) repository.KeyboardMaterialColor {
	if m == nil {
		return repository.KeyboardMaterialColor{}
	}

	return repository.KeyboardMaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func keyboardDesignToAPI(d repository.KeyboardDesign) *api.KeyboardDesign {
	topCase := keyboardMaterialColorToAPI(d.TopCase)
	bottomCase := keyboardMaterialColorToAPI(d.BottomCase)
	weight := keyboardMaterialColorToAPI(d.Weight)
	if topCase == nil && bottomCase == nil && weight == nil && d.Plates == nil {
		return nil
	}

	var plates *[]string
	if d.Plates != nil {
		plates = &d.Plates
	}

	return &api.KeyboardDesign{
		TopCase:    topCase,
		BottomCase: bottomCase,
		Weight:     weight,
		Plates:     plates,
	}
}

func keyboardDesignToRepo(d *api.KeyboardDesign) repository.KeyboardDesign {
	if d == nil {
		return repository.KeyboardDesign{}
	}

	out := repository.KeyboardDesign{
		TopCase:    keyboardMaterialColorToRepo(d.TopCase),
		BottomCase: keyboardMaterialColorToRepo(d.BottomCase),
		Weight:     keyboardMaterialColorToRepo(d.Weight),
	}
	if d.Plates != nil {
		out.Plates = *d.Plates
	}

	return out
}

func keyboardPCBToAPI(p repository.KeyboardPCB) *api.KeyboardPCB {
	if p.Thickness == nil && p.Firmware == nil && p.Assembly == nil && p.Connectivity == nil {
		return nil
	}

	return &api.KeyboardPCB{
		Thickness:    p.Thickness,
		Firmware:     p.Firmware,
		Assembly:     p.Assembly,
		Connectivity: p.Connectivity,
	}
}

func keyboardPCBToRepo(p *api.KeyboardPCB) repository.KeyboardPCB {
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

// dateLayout matches how openapi_types.Date marshals/unmarshals.
const dateLayout = "2006-01-02"

func keyboardPurchaseToAPI(p repository.KeyboardPurchase, isOwner bool) (*api.Purchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
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

func parseAPIDate(s string) (*openapi_types.Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("stored date %q does not match layout %q: %w", s, dateLayout, err)
	}

	return &openapi_types.Date{Time: t}, nil
}

func keyboardPurchaseToRepo(p *api.Purchase) repository.KeyboardPurchase {
	if p == nil {
		return repository.KeyboardPurchase{}
	}

	out := repository.KeyboardPurchase{
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
