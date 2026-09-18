package repomcp

import (
	"context"
	"errors"
	"fmt"

	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// Build maps repository.Build to and from its MCP tool shape.
type Build struct {
	KeyboardRepo repository.KeyboardRepository
}

// ToMCP never presigns an image URL, unlike
// [github.com/rogueserenity/kbdb/internal/repoapi.Build.ToAPI] - it reports
// only HasImages, so this can't fail on a presign error. The owner always
// sees their own Stabs.Price; a non-owner sees it only if
// ownerPrefs.ShowPriceToOthers.
func (b Build) ToMCP(build repository.Build, isOwner bool, ownerPrefs repository.ProfilePreferences) schema.Build {
	return schema.Build{
		ID:            build.ID,
		Keyboard:      build.Keyboard,
		Plate:         build.Plate,
		CaseMountType: b.caseMountTypeToMCP(build.CaseMountType),
		Stabs:         b.stabsToMCP(build.Stabs, ownerPrefs.ShowPriceSingle(isOwner)),
		Foam:          build.Foam,
		Switches:      b.switchEntriesToMCP(build.Switches),
		KeycapKits:    b.keycapKitEntriesToMCP(build.KeycapKits),
		BuildDate:     build.BuildDate,
		Notes:         build.Notes,
		Visibility:    string(build.Visibility),
		HasImages:     len(build.Images) > 0,
	}
}

// FromMCP leaves ID and UserID unset: the caller sets ID, and UserID comes
// from ctx in the repository layer. Images are left unset too - never
// carried in a build write, managed one at a time via their own tools.
func (b Build) FromMCP(in schema.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		Plate:         in.Plate,
		CaseMountType: b.caseMountTypeFromMCP(in.CaseMountType),
		Stabs:         b.stabsFromMCP(in.Stabs),
		Foam:          in.Foam,
		Switches:      b.switchEntriesFromMCP(in.Switches),
		KeycapKits:    b.keycapKitEntriesFromMCP(in.KeycapKits),
		BuildDate:     in.BuildDate,
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

// ToMCPSummary mirrors [github.com/rogueserenity/kbdb/internal/repoapi.Build.ToAPISummary]'s
// KeyboardRepo.Get denormalization but reports HasImage rather than a
// presigned URL.
func (b Build) ToMCPSummary(ctx context.Context, build repository.Build) (schema.BuildSummary, error) {
	summary := schema.BuildSummary{
		ID:         build.ID,
		KeyboardID: build.Keyboard,
		BuildDate:  build.BuildDate,
		HasImage:   len(build.Images) > 0,
	}

	kb, err := b.KeyboardRepo.Get(ctx, build.UserID, build.Keyboard)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return schema.BuildSummary{}, fmt.Errorf("getting keyboard %q for build %q: %w", build.Keyboard, build.ID, err)
		}
		// Leave summary.Keyboard nil.
	} else {
		summary.Keyboard = &schema.BuildSummaryKeyboard{Brand: kb.Brand, Name: kb.Name}
	}

	return summary, nil
}

func (b Build) caseMountTypeToMCP(cmt *repository.BuildCaseMountType) *schema.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &schema.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) caseMountTypeFromMCP(cmt *schema.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func (b Build) stabsToMCP(s *repository.BuildStabs, showPrice bool) *schema.BuildStabs {
	if s == nil {
		return nil
	}

	out := &schema.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
	}
	if showPrice {
		out.Price = s.Price
	}

	return out
}

func (b Build) stabsFromMCP(s *schema.BuildStabs) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func (b Build) switchEntriesToMCP(entries []repository.BuildSwitchEntry) []schema.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) switchEntriesFromMCP(entries []schema.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func (b Build) keycapKitEntriesToMCP(entries []repository.BuildKeycapKitEntry) []schema.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

func (b Build) keycapKitEntriesFromMCP(entries []schema.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}
