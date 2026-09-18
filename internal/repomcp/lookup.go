package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
)

// Lookup maps lookup.Lookup to its MCP tool shape. It has no dependencies -
// lookups are static, deploy-time data.
type Lookup struct{}

// ToMCP maps a lookup.Lookup to its wire representation.
func (Lookup) ToMCP(l lookup.Lookup) schema.GetLookupOutput {
	return schema.GetLookupOutput{
		Category: string(l.Category),
		Values:   l.Values,
	}
}
