package repomcp

import (
	"github.com/rogueserenity/kbdb/internal/lookup"
	"github.com/rogueserenity/kbdb/internal/mcp/schema"
)

// LookupToMCP maps a lookup.Lookup to its wire representation.
func LookupToMCP(l lookup.Lookup) schema.GetLookupOutput {
	return schema.GetLookupOutput{
		Category: string(l.Category),
		Values:   l.Values,
	}
}
