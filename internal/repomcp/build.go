package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
	"github.com/rogueserenity/kbdb/internal/repository"
)

// BuildToMCP maps a repository.Build to its MCP tool shape. Unlike
// repoapi.BuildToAPI, this never presigns a GET URL for an image - it
// reports only HasImages, so mapping a build can't fail the way the REST
// mapping can on a presign error.
func BuildToMCP(b repository.Build) schema.Build {
	return schema.Build{
		ID:            b.ID,
		Keyboard:      b.Keyboard,
		Plate:         b.Plate,
		CaseMountType: buildCaseMountTypeToMCP(b.CaseMountType),
		Stabs:         buildStabsToMCP(b.Stabs),
		Foam:          b.Foam,
		Switches:      buildSwitchEntriesToMCP(b.Switches),
		KeycapKits:    buildKeycapKitEntriesToMCP(b.KeycapKits),
		BuildDate:     b.BuildDate,
		Notes:         b.Notes,
		Visibility:    string(b.Visibility),
		HasImages:     len(b.Images) > 0,
	}
}

// BuildFromMCP maps a create_build tool argument to its repository shape.
// ID and UserID are left unset: the caller sets ID, and UserID comes from
// ctx in the repository layer. Images are left unset too - a build write
// never carries images, which are managed one at a time via their own
// tools.
func BuildFromMCP(in schema.BuildInput) repository.Build {
	return repository.Build{
		Keyboard:      in.Keyboard,
		Plate:         in.Plate,
		CaseMountType: buildCaseMountTypeFromMCP(in.CaseMountType),
		Stabs:         buildStabsFromMCP(in.Stabs),
		Foam:          in.Foam,
		Switches:      buildSwitchEntriesFromMCP(in.Switches),
		KeycapKits:    buildKeycapKitEntriesFromMCP(in.KeycapKits),
		BuildDate:     in.BuildDate,
		Notes:         in.Notes,
		Visibility:    repository.Visibility(in.Visibility),
	}
}

func buildCaseMountTypeToMCP(cmt *repository.BuildCaseMountType) *schema.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &schema.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildCaseMountTypeFromMCP(cmt *schema.BuildCaseMountType) *repository.BuildCaseMountType {
	if cmt == nil {
		return nil
	}

	return &repository.BuildCaseMountType{
		Type:      cmt.Type,
		Durometer: cmt.Durometer,
	}
}

func buildStabsToMCP(s *repository.BuildStabs) *schema.BuildStabs {
	if s == nil {
		return nil
	}

	return &schema.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func buildStabsFromMCP(s *schema.BuildStabs) *repository.BuildStabs {
	if s == nil {
		return nil
	}

	return &repository.BuildStabs{
		Name:      s.Name,
		MountType: s.MountType,
		Price:     s.Price,
	}
}

func buildSwitchEntriesToMCP(entries []repository.BuildSwitchEntry) []schema.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func buildSwitchEntriesFromMCP(entries []schema.BuildSwitchEntry) []repository.BuildSwitchEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildSwitchEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildSwitchEntry{Switch: e.Switch, Count: e.Count}
	}

	return out
}

func buildKeycapKitEntriesToMCP(entries []repository.BuildKeycapKitEntry) []schema.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]schema.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = schema.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}

func buildKeycapKitEntriesFromMCP(entries []schema.BuildKeycapKitEntry) []repository.BuildKeycapKitEntry {
	if entries == nil {
		return nil
	}

	out := make([]repository.BuildKeycapKitEntry, len(entries))
	for i, e := range entries {
		out[i] = repository.BuildKeycapKitEntry{KeycapSet: e.KeycapSet, Kit: e.Kit}
	}

	return out
}
