// Package repository defines, per entity, the data-model struct and the
// interface of operations available on it. No AWS/DynamoDB SDK types here —
// concrete implementations live in subpackages (e.g. internal/repository/dynamo).
package repository

import "context"

// Lookup is a lookup category's approved values. values is usually a list
// of plain strings, but some categories store a list of objects instead —
// see model/lookup_seed.json.
type Lookup struct {
	Category string `dynamodbav:"category" json:"category"`
	Values   []any  `dynamodbav:"values" json:"values"`
}

// LookupRepository provides access to lookup categories.
type LookupRepository interface {
	ListCategories(ctx context.Context) ([]string, error)
	GetCategory(ctx context.Context, category string) (*Lookup, error)
	CreateCategory(ctx context.Context, category string, values []any) (*Lookup, error)
	ReplaceCategory(ctx context.Context, category string, values []any) (*Lookup, error)
	DeleteCategory(ctx context.Context, category string) error
}
