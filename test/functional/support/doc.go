// Package support holds env-var-derived configuration shared by every
// functional test suite (base URL, table names, mockoidc credentials) - the
// common ground support/api's HTTP clients and support/db's DynamoDB
// seed/cleanup helpers both build on, so neither has to import the other.
package support
