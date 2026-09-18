package repoapi

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// Keyboard maps repository.Keyboard to and from its wire representations.
type Keyboard struct {
	Images     repository.KeyboardImageStore
	Repo       repository.KeyboardRepository
	PresignTTL time.Duration
}

// ToAPI maps a repository.Keyboard to its wire representation. The owner
// always sees their own purchase.price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers. The rest of purchase is unaffected. Returns
// an error if a stored Purchase date doesn't match dateLayout, or an image
// fails to presign.
func (k Keyboard) ToAPI(ctx context.Context, kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.Keyboard, error) {
	purchase, err := k.purchaseToAPI(kb.Purchase, ownerPrefs.ShowPriceSingle(isOwner))
	if err != nil {
		return api.Keyboard{}, err
	}

	imgs, err := k.imagesToAPI(ctx, kb.UserID, kb.ID, repository.SortedKeyboardImages(kb.Images))
	if err != nil {
		return api.Keyboard{}, err
	}

	return api.Keyboard{
		Id:         kb.ID,
		Brand:      kb.Brand,
		Name:       kb.Name,
		Size:       kb.Size,
		Layout:     kb.Layout,
		Design:     k.designToAPI(kb.Design),
		Pcb:        k.pcbToAPI(kb.PCB),
		Purchase:   purchase,
		Notes:      kb.Notes,
		Visibility: api.Visibility(kb.Visibility),
		Images:     imgs,
	}, nil
}

// imagesToAPI resolves a presigned GET URL per image, reusing each image's
// cached URL if still fresh, mirroring [Build.imagesToAPI]. images is
// already ordered (by Seq) by the caller.
func (k Keyboard) imagesToAPI(ctx context.Context, ownerID, keyboardID string, images []repository.KeyboardImage) (*[]api.KeyboardImage, error) {
	if len(images) == 0 {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.KeyboardImage, len(images))
	errs := make([]error, len(images))

	var wg sync.WaitGroup
	for i, img := range images {
		wg.Add(1)
		go func(i int, img repository.KeyboardImage) {
			defer wg.Done()

			url, err := k.resolveKeyboardImageURL(ctx, ownerID, keyboardID, img)
			if err != nil {
				errs[i] = fmt.Errorf("presigning keyboard image %q: %w", img.ImageID, err)
				return
			}
			out[i] = api.KeyboardImage{ImageId: img.ImageID, Url: url}
		}(i, img)
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return &out, nil
}

// ToRepo maps a generated KeyboardInput (already schema-validated by the
// OpenAPI request validator) to a repository.Keyboard. It does not set
// UserID or ID - those come from the request's path/caller, not the body,
// and stay the handler's responsibility.
func (k Keyboard) ToRepo(in api.KeyboardInput) repository.Keyboard {
	return repository.Keyboard{
		Brand:      in.Brand,
		Name:       in.Name,
		Size:       in.Size,
		Layout:     in.Layout,
		Design:     k.designToRepo(in.Design),
		PCB:        k.pcbToRepo(in.Pcb),
		Purchase:   k.purchaseToRepo(in.Purchase),
		Notes:      in.Notes,
		Visibility: repository.Visibility(in.Visibility),
	}
}

// ToAPISummary maps a repository.Keyboard to the KeyboardSummary schema
// returned by the list endpoint. Image is the first entry of Images,
// presigned, if any - mirrors [Build.ToAPISummary]'s handling of a build's
// images. Price is shown per ownerPrefs.ShowPriceToMe (owner) or
// ownerPrefs.ShowPriceToOthers (non-owner) - unlike [Keyboard.ToAPI], the
// owner isn't unconditionally shown price here.
func (k Keyboard) ToAPISummary(ctx context.Context, kb repository.Keyboard, isOwner bool, ownerPrefs repository.ProfilePreferences) (api.KeyboardSummary, error) {
	var image *api.KeyboardImage
	if first := repository.SortedKeyboardImages(kb.Images); len(first) > 0 {
		img := first[0]
		url, err := k.resolveKeyboardImageURL(ctx, kb.UserID, kb.ID, img)
		if err != nil {
			return api.KeyboardSummary{}, fmt.Errorf("presigning keyboard image %q: %w", img.ImageID, err)
		}
		image = &api.KeyboardImage{ImageId: img.ImageID, Url: url}
	}

	summary := api.KeyboardSummary{
		Id:          &kb.ID,
		Brand:       &kb.Brand,
		Name:        &kb.Name,
		Size:        kb.Size,
		Layout:      kb.Layout,
		OrderStatus: kb.Purchase.OrderStatus,
		Image:       image,
	}
	if ownerPrefs.ShowPriceSummary(isOwner) {
		summary.Price = kb.Purchase.Price
	}

	return summary, nil
}

// resolveKeyboardImageURL presigns img.Path, reusing its cached GET URL if
// still fresh enough.
func (k Keyboard) resolveKeyboardImageURL(ctx context.Context, ownerID, keyboardID string, img repository.KeyboardImage) (string, error) {
	return resolveImageURL(img.GetURL, img.GetURLExpiresAt, k.PresignTTL,
		func() (string, error) { return k.Images.PresignGetKeyboardImage(ctx, img.Path) },
		func(url string, expiresAt time.Time) error {
			_, err := k.Repo.SetImageGetCache(ctx, ownerID, keyboardID, img.ImageID, img.Path, url, expiresAt)
			return err
		},
	)
}

func (k Keyboard) materialColorToAPI(m repository.KeyboardMaterialColor) *api.MaterialColor {
	if m.Material == nil && m.Color == nil {
		return nil
	}

	return &api.MaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func (k Keyboard) materialColorToRepo(m *api.MaterialColor) repository.KeyboardMaterialColor {
	if m == nil {
		return repository.KeyboardMaterialColor{}
	}

	return repository.KeyboardMaterialColor{
		Material: m.Material,
		Color:    m.Color,
	}
}

func (k Keyboard) designToAPI(d repository.KeyboardDesign) *api.KeyboardDesign {
	topCase := k.materialColorToAPI(d.TopCase)
	bottomCase := k.materialColorToAPI(d.BottomCase)
	weight := k.materialColorToAPI(d.Weight)
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

func (k Keyboard) designToRepo(d *api.KeyboardDesign) repository.KeyboardDesign {
	if d == nil {
		return repository.KeyboardDesign{}
	}

	out := repository.KeyboardDesign{
		TopCase:    k.materialColorToRepo(d.TopCase),
		BottomCase: k.materialColorToRepo(d.BottomCase),
		Weight:     k.materialColorToRepo(d.Weight),
	}
	if d.Plates != nil {
		out.Plates = *d.Plates
	}

	return out
}

func (k Keyboard) pcbToAPI(p repository.KeyboardPCB) *api.KeyboardPCB {
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

func (k Keyboard) pcbToRepo(p *api.KeyboardPCB) repository.KeyboardPCB {
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

func (k Keyboard) purchaseToAPI(p repository.KeyboardPurchase, showPrice bool) (*api.Purchase, error) {
	if p.Vendor == nil && p.Price == nil && p.OrderDate == nil && p.DeliveryDate == nil && p.OrderStatus == nil {
		return nil, nil //nolint:nilnil // no purchase data is a valid, expected result
	}

	out := &api.Purchase{
		Vendor:      p.Vendor,
		OrderStatus: p.OrderStatus,
	}
	if showPrice {
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

func (k Keyboard) purchaseToRepo(p *api.Purchase) repository.KeyboardPurchase {
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
