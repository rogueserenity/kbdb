// Package repository defines, per entity, the data-model struct and the
// interface of operations available on it. No AWS/DynamoDB SDK types here —
// concrete implementations live in subpackages (e.g. internal/repository/dynamo).
package repository

import "context"

// Lookup is a lookup category's approved values. values is usually a list
// of plain strings, but some categories store a list of objects instead —
// see model/lookup_seed.json.
type Lookup struct {
	PK     string `dynamodbav:"PK"`
	Values []any  `dynamodbav:"values"`
}

// LookupRepository provides access to lookup categories.
type LookupRepository interface {
	ListCategories(ctx context.Context) ([]string, error)
}
