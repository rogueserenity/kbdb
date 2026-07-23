// Package fixtures holds the fixed test values the standalone mockoidc
// server (cmd/mockoidc, package main, not importable) is configured with,
// so both that program and functional-test specs reference the same
// constants rather than duplicating them.
package fixtures

// TestUserSubject is the "sub" claim of the plain (non-admin) test user,
// matching what auth.Claims.Subject will resolve to for tokens minted by
// mockoidc via support.AuthToken.
const TestUserSubject = "test-user-0001"

// AdminUserSubject is the "sub" claim of the admin test user, minted by
// support.AdminAuthToken.
const AdminUserSubject = "test-admin-0001"

// AdminGroups is the cognito:groups claim value admin-flavored test tokens
// carry - matches the "admins" group template.yaml's AdminsGroup declares
// (see internal/auth.Claims.Groups).
var AdminGroups = []string{"admins"}

// TestClientID/TestClientSecret are fixed so specs and the local env-vars
// file can reference known values; deliberately low-entropy/fake-looking so
// a real credential pasted in here later would stand out.
const (
	TestClientID     = "no-client-id-here-ok"
	TestClientSecret = "no-secret-here-ok"
)
