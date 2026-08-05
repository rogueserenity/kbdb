// Package repository defines, per entity, the data-model struct and the
// interface of operations available on it. No AWS/DynamoDB SDK types here —
// concrete implementations live in subpackages (e.g. internal/repository/dynamo).
package repository
