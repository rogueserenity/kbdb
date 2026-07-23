// Package ctx holds request-scoped identity values carried through
// context.Context, as plain retrievable types — independent of logging (see
// internal/log) or any particular transport (REST, MCP). Business logic
// that needs to know the current caller's user ID depends on this package
// directly, not on internal/log's request-logging plumbing.
package ctx

import "context"

type userIDKey struct{}

// WithUserID returns a context carrying userID as the current caller's
// identity.
func WithUserID(c context.Context, userID string) context.Context {
	return context.WithValue(c, userIDKey{}, userID)
}

// UserID returns the current caller's user ID, if one has been set.
func UserID(c context.Context) (string, bool) {
	v, ok := c.Value(userIDKey{}).(string)
	return v, ok
}

type groupsKey struct{}

// WithGroups returns a context carrying groups as the current caller's
// cognito:groups claim.
func WithGroups(c context.Context, groups []string) context.Context {
	return context.WithValue(c, groupsKey{}, groups)
}

// Groups returns the current caller's cognito:groups claim, or nil if none
// has been set.
func Groups(c context.Context) []string {
	v, _ := c.Value(groupsKey{}).([]string)
	return v
}
