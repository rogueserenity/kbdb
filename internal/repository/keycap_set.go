package repository

import "context"

// KeycapKitPurchase tracks a kit's own purchase lifecycle, independent of
// the parent set's (a set can be assembled from kits bought separately).
type KeycapKitPurchase struct {
	Vendor       *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price        *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate    *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	OrderStatus  *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
}

// KeycapKit is one purchase within a KeycapSet (e.g. "Base", "Extension").
// KitID is server-generated and unique within its parent set, not globally.
type KeycapKit struct {
	KitID     string            `dynamodbav:"kit_id" json:"kit_id"`
	Name      string            `dynamodbav:"name" json:"name"`
	ImagePath *string           `dynamodbav:"image_path,omitempty" json:"image_path,omitempty"`
	Purchase  KeycapKitPurchase `dynamodbav:"purchase" json:"purchase"`
}

// KeycapSet is a keycap set in a user's collection, or shared with the
// caller. UserID is the DynamoDB partition key (the owner's Cognito
// subject); ID is the sort key. Only Brand, Name, and Visibility are
// required, per api/openapi.yaml's KeycapSetInput schema.
type KeycapSet struct {
	UserID     string      `dynamodbav:"user_id" json:"-"`
	ID         string      `dynamodbav:"id" json:"id"`
	Brand      string      `dynamodbav:"brand" json:"brand"`
	Name       string      `dynamodbav:"name" json:"name"`
	Profile    *string     `dynamodbav:"profile,omitempty" json:"profile,omitempty"`
	Material   *string     `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Notes      *string     `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility  `dynamodbav:"visibility" json:"visibility"`
	Kits       []KeycapKit `dynamodbav:"kits,omitempty" json:"kits,omitempty"`
	// Version guards kit sub-mutations against a lost update when two
	// concurrent calls read-modify-write this item's Kits slice - not
	// exposed via the API, purely a repository-internal CAS mechanism.
	Version int `dynamodbav:"version" json:"-"`
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

	// AddKit appends kit to the set's Kits (kit.KitID must already be set)
	// and returns the stored kit. Performs a read-modify-write of the whole
	// parent item under a bounded version-CAS retry loop - not an
	// independent-table operation - but that's an implementation detail:
	// callers get back just the kit they created, matching Create's shape
	// for every other entity. Returns ErrNotFound if the parent set doesn't
	// exist.
	AddKit(ctx context.Context, setID string, kit KeycapKit) (*KeycapKit, error)
}
