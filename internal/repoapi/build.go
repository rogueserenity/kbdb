package repoapi

import (
	"context"
	"errors"
	"fmt"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/rogueserenity/kbdb/internal/handlers/api"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// BuildToAPI maps a repository.Build to its wire representation. Returns an
// error if b.BuildDate doesn't match dateLayout, or if an image's Path fails
// to presign via images.PresignGetBuildImage.
func BuildToAPI(ctx context.Context, b repository.Build, images repository.BuildImageStore) (api.Build, error) {
	buildDate, err := buildDateToAPI(b.BuildDate)
	if err != nil {
		return api.Build{}, err
	}

	imgs, err := buildImagesToAPI(ctx, b.Images, images)
	if err != nil {
		return api.Build{}, err
	}

	return api.Build{
		Id:            b.ID,
		Keyboard:      b.Keyboard,
		Plate:         b.Plate,
		CaseMountType: buildCaseMountTypeToAPI(b.CaseMountType),
		Stabs:         buildStabsToAPI(b.Stabs),
		Foam:          b.Foam,
		Switches:      buildSwitchEntriesToAPI(b.Switches),
		KeycapKits:    buildKeycapKitEntriesToAPI(b.KeycapKits),
		BuildDate:     buildDate,
		Notes:         b.Notes,
		Visibility:    api.Visibility(b.Visibility),
		Images:        imgs,
	}, nil
}

// BuildToRepo maps a generated BuildInput (already schema-validated by the
// OpenAPI request validator) to a repository.Build. It does not set UserID
// or ID - those come from the request's path/caller, not the body, and stay
// the handler's responsibility. Images is never set here - a build's images
// are managed entirely through the dedicated image endpoints, never carried
// in a build write.
func BuildToRepo(in api.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		Plate:         in.Plate,
		CaseMountType: buildCaseMountTypeToRepo(in.CaseMountType),
		Stabs:         buildStabsToRepo(in.Stabs),
		Foam:          in.Foam,
		Switches:      buildSwitchEntriesToRepo(in.Switches),
		KeycapKits:    buildKeycapKitEntriesToRepo(in.KeycapKits),
		BuildDate:     buildDateToRepo(in.BuildDate),
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// BuildToAPISummary denormalizes the referenced Keyboard's brand/name via
// a per-item keyboardRepo.Get call - no batch-get precedent exists, and a
// list page is capped at 100 items, so this O(n) fetch is the simplest
// correct approach for now. If the keyboard can't be resolved
// (repository.ErrNotFound - e.g. deleted after the build was created, see
// https://github.com/rogueserenity/kbdb/issues/172), Keyboard is left nil
// rather than failing the whole request; any other error still fails it.
func BuildToAPISummary(ctx context.Context, b repository.Build, keyboardRepo repository.KeyboardRepository, images repository.BuildImageStore) (api.BuildSummary, error) {
	buildDate, err := buildDateToAPI(b.BuildDate)
	if err != nil {
		return api.BuildSummary{}, err
	}

	var image *api.BuildImage
	if len(b.Images) > 0 {
		url, err := images.PresignGetBuildImage(ctx, b.Images[0].Path)
		if err != nil {
			return api.BuildSummary{}, fmt.Errorf("presigning build image %q: %w", b.Images[0].ImageID, err)
		}
		image = &api.BuildImage{ImageId: b.Images[0].ImageID, Url: url}
	}

	summary := api.BuildSummary{
		Id:        &b.ID,
		BuildDate: buildDate,
		Image:     image,
	}

	kb, err := keyboardRepo.Get(ctx, b.UserID, b.Keyboard)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return api.BuildSummary{}, fmt.Errorf("getting keyboard %q for build %q: %w", b.Keyboard, b.ID, err)
		}
		// Leave summary.Keyboard nil.
	} else {
		summary.Keyboard = &api.BuildSummaryKeyboard{Brand: &kb.Brand, Name: &kb.Name}
	}

	return summary, nil
}

func buildDateToAPI(s *string) (*openapi_types.Date, error) {
	if s == nil {
		return nil, nil //nolint:nilnil // no build date is a valid, expected result
	}

	d, err := parseAPIDate(*s)
	if err != nil {
		return nil, fmt.Errorf("parsing build_date: %w", err)
	}

	return d, nil
}

func buildDateToRepo(d *openapi_types.Date) *string {
	if d == nil {
		return nil
	}

	s := d.Format(dateLayout)
	return &s
}

func buildCaseMountTypeToAPI(cmt *repository.BuildCaseMountType) *api.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &api.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildCaseMountTypeToRepo(cmt *api.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildStabsToAPI(s *repository.BuildStabs) *api.BuildStabs {
	if s == nil {
		return nil
	}

	return &api.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func buildStabsToRepo(s *api.BuildStabs) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func buildSwitchEntriesToAPI(entries []repository.BuildSwitchEntry) *[]api.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]api.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = api.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return &out
}

func buildSwitchEntriesToRepo(entries *[]api.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func buildKeycapKitEntriesToAPI(entries []repository.BuildKeycapKitEntry) *[]api.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]api.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = api.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return &out
}

func buildKeycapKitEntriesToRepo(entries *[]api.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(*entries))
	for i, e := range *entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

// buildImagesToAPI mints a fresh presigned GET URL per image, per request -
// never persisted, mirroring [KeycapKitToAPI]'s handling of a kit's image.
func buildImagesToAPI(ctx context.Context, images []repository.BuildImage, store repository.BuildImageStore) (*[]api.BuildImage, error) {
	if images == nil {
		return nil, nil //nolint:nilnil // no images is a valid, expected result
	}

	out := make([]api.BuildImage, len(images))
	for i, img := range images {
		url, err := store.PresignGetBuildImage(ctx, img.Path)
		if err != nil {
			return nil, fmt.Errorf("presigning build image %q: %w", img.ImageID, err)
		}
		out[i] = api.BuildImage{ImageId: img.ImageID, Url: url}
	}

	return &out, nil
}
