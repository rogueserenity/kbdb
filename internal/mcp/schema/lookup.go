package schema

// ListLookupsInput is empty: list_lookups takes no arguments. A named type
// is still required for mcp.AddTool to infer the tool's input schema.
type ListLookupsInput struct{}

// ListLookupsOutput is the list_lookups tool result.
type ListLookupsOutput struct {
	Categories []string `json:"categories" jsonschema:"every lookup category name"`
}

// GetLookupInput is the get_lookup tool argument.
type GetLookupInput struct {
	Category string `json:"category" jsonschema:"the lookup category name, e.g. switch_type or vendor"`
}

// GetLookupOutput is the get_lookup tool result.
type GetLookupOutput struct {
	Category string `json:"category" jsonschema:"the lookup category name"`
	Values   []any  `json:"values" jsonschema:"the category's approved values"`
}
