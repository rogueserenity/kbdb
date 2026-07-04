// Package fixtures holds the fixed test values the standalone mockoidc
// server (cmd/mockoidc, package main, not importable) is configured with,
// so both that program and functional-test specs reference the same
// constants rather than duplicating them.
package fixtures

// TestUserSubject is the "sub" claim of the seeded test user, matching what
// auth.Claims.Subject will resolve to for tokens minted by mockoidc.
const TestUserSubject = "test-user-0001"

// TestClientID/TestClientSecret are fixed (not randomly generated) so
// functional specs and sam local start-api's env-vars file can reference
// known values, rather than needing to discover mockoidc.NewServer's
// randomly-generated ones out of band at runtime.
const (
	TestClientID     = "kbdb-func-test-client"
	TestClientSecret = "kbdb-func-test-secret"
)
