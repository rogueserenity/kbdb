package schema

type ListLookupsInput struct{}

type ListLookupsOutput struct {
	Categories []string `json:"categories" jsonschema:"every lookup category name"`
}

type GetLookupInput struct {
	Category string `json:"category" jsonschema:"the lookup category name, e.g. switch_type or vendor"`
}

type GetLookupOutput struct {
	Category string `json:"category" jsonschema:"the lookup category name"`
	Values   []any  `json:"values" jsonschema:"the category's approved values"`
}
