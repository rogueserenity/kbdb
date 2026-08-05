package fixtures

// TestUserSubject is the "sub" claim of the plain (non-admin) test user,
// matching what auth.Claims.Subject will resolve to for tokens minted by
// mockoidc via support.AuthToken.
const TestUserSubject = "test-user-0001"

// AdminUserSubject is the "sub" claim of the admin test user, minted by
// support.AdminAuthToken.
const AdminUserSubject = "test-admin-0001"

// SecondUserSubject is the "sub" claim of a second, unrelated plain test
// user, minted by support.SecondUserAuthToken - used to exercise
// visibility-scoped reads of another user's items (see internal/authz)
// where TestUserSubject and AdminUserSubject aren't a fit: authorization
// there is by ownership, not admin membership.
const SecondUserSubject = "test-user-0002"

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
