package repository

import "context"

// KeycapSet is a keycap set in a user's collection, or shared with the
// caller. UserID is the DynamoDB partition key (the owner's Cognito
// subject); ID is the sort key. Only Brand, Name, and Visibility are
// required, per api/openapi.yaml's KeycapSetInput schema.
type KeycapSet struct {
	UserID     string     `dynamodbav:"user_id" json:"-"`
	ID         string     `dynamodbav:"id" json:"id"`
	Brand      string     `dynamodbav:"brand" json:"brand"`
	Name       string     `dynamodbav:"name" json:"name"`
	Profile    *string    `dynamodbav:"profile,omitempty" json:"profile,omitempty"`
	Material   *string    `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Notes      *string    `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility `dynamodbav:"visibility" json:"visibility"`
}

// KeycapSetRepository provides access to keycap sets.
type KeycapSetRepository interface {
	// List returns up to limit keycap sets owned by ownerID whose
	// Visibility is in visibilities, ordered by ID. cursor, if non-empty,
	// resumes from a previous call's returned cursor; the returned cursor
	// is empty when there are no more pages.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (sets []KeycapSet, nextCursor string, err error)

	// Get returns the keycap set owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via internal/authz.CanReadVisibility.
	Get(ctx context.Context, ownerID, id string) (*KeycapSet, error)

	// Create stores ks (UserID is set from ctx, ks.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Update replaces the keycap set with ks.ID (UserID is set from ctx,
	// ks.ID must already be set to the keycap set being updated). Returns
	// ErrNotFound if no keycap set with that ID exists for the owner.
	Update(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Delete removes the caller's keycap set with the given id. Idempotent:
	// a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error
}
