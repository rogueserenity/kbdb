package repository

import (
	"context"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

// SwitchMaterial is the housing/stem material makeup of a switch.
type SwitchMaterial struct {
	TopHousing    *string `dynamodbav:"top_housing,omitempty" json:"top_housing,omitempty"`
	BottomHousing *string `dynamodbav:"bottom_housing,omitempty" json:"bottom_housing,omitempty"`
	Stem          *string `dynamodbav:"stem,omitempty" json:"stem,omitempty"`
}

// SwitchForce is a switch's nominal actuation/bottom-out force, in grams.
type SwitchForce struct {
	Actuation *float64 `dynamodbav:"actuation,omitempty" json:"actuation,omitempty"`
	BottomOut *float64 `dynamodbav:"bottom_out,omitempty" json:"bottom_out,omitempty"`
}

// SwitchSpring is a switch's spring material and travel distances (mm).
type SwitchSpring struct {
	Material    *string  `dynamodbav:"material,omitempty" json:"material,omitempty"`
	PreTravel   *float64 `dynamodbav:"pre_travel,omitempty" json:"pre_travel,omitempty"`
	TotalTravel *float64 `dynamodbav:"total_travel,omitempty" json:"total_travel,omitempty"`
}

// SwitchPurchase is where/how much/how many of a switch were bought, plus
// its order lifecycle. Same shape as KeyboardPurchase, plus Quantity since
// switches are bought in bulk.
type SwitchPurchase struct {
	Vendor       *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price        *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate    *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	OrderStatus  *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
	Quantity     *int     `dynamodbav:"quantity,omitempty" json:"quantity,omitempty"`
}

// Switch is a mechanical keyboard switch in a user's collection, or shared
// with the caller. UserID is the DynamoDB partition key (the owner's
// IdP-issued user ID); ID is the sort key. Only Brand, Name, Type, and
// Visibility are required, per api/openapi.yaml's SwitchInput schema; every
// other field (here and in SwitchMaterial/SwitchForce/SwitchSpring/
// SwitchPurchase) is a pointer so nil ("not provided") round-trips
// distinctly from an explicit zero value.
type Switch struct {
	UserID       string          `dynamodbav:"user_id" json:"-"`
	ID           string          `dynamodbav:"id" json:"id"`
	Brand        string          `dynamodbav:"brand" json:"brand"`
	Manufacturer *string         `dynamodbav:"manufacturer,omitempty" json:"manufacturer,omitempty"`
	Name         string          `dynamodbav:"name" json:"name"`
	Type         string          `dynamodbav:"type" json:"type"`
	Pins         *int            `dynamodbav:"pins,omitempty" json:"pins,omitempty"`
	FactoryLubed *bool           `dynamodbav:"factory_lubed,omitempty" json:"factory_lubed,omitempty"`
	Material     SwitchMaterial  `dynamodbav:"material" json:"material"`
	Force        SwitchForce     `dynamodbav:"force" json:"force"`
	Spring       SwitchSpring    `dynamodbav:"spring" json:"spring"`
	Purchase     SwitchPurchase  `dynamodbav:"purchase" json:"purchase"`
	Notes        *string         `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility   Visibility      `dynamodbav:"visibility" json:"visibility"`
	ImagePath    *SwitchImageKey `dynamodbav:"image_path,omitempty" json:"-"`
	// Version is a repository-internal monotonic write counter, bumped by
	// every Update, not exposed via the API. Writes use conditional
	// UpdateItem for update-only semantics rather than reading this back for
	// CAS, but it stays as the hook for a future full-body re-merge.
	Version int `dynamodbav:"version" json:"-"`
}

// SwitchRepository provides access to switches. List/Get take an explicit
// ownerID since reads can target another user's shared items;
// Create/Update/Delete read the caller from ctx (internal/ctx.UserID)
// instead, since writes are always self-scoped.
type SwitchRepository interface {
	// List returns up to limit switches owned by ownerID whose Visibility is
	// in visibilities, ordered by ID. cursor, if non-empty, resumes from a
	// previous call's returned cursor; the returned cursor is empty when
	// there are no more pages.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (switches []Switch, nextCursor string, err error)

	// Get returns the switch owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via
	// [github.com/rogueserenity/kbdb/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*Switch, error)

	// Create stores sw (UserID is set from ctx, sw.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, sw Switch) (*Switch, error)

	// Update replaces the caller's switch (UserID is set from ctx, sw.ID
	// must already be set to the switch being updated). Returns ErrNotFound
	// if no switch with that id exists for the caller.
	Update(ctx context.Context, sw Switch) (*Switch, error)

	// Delete removes the caller's switch with the given id. Callers clean
	// up any image it had in a SwitchImageStore themselves, before calling
	// Delete. Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// SetImagePath sets the caller's switch's ImagePath with a single
	// conditional UpdateItem. Returns ErrNotFound if id doesn't exist.
	SetImagePath(ctx context.Context, id string, key SwitchImageKey) error

	// ClearImagePath clears the caller's switch's ImagePath and returns the
	// key that was cleared, or nil if it was already unset. Idempotent: a
	// switch with no ImagePath already set is not an error. Returns
	// ErrNotFound if id doesn't exist.
	ClearImagePath(ctx context.Context, id string) (*SwitchImageKey, error)
}

// SwitchImageKey is the object key a switch's image is stored under in a
// SwitchImageStore.
type SwitchImageKey string

// NewSwitchImageKey builds the deterministic object key for switchID's
// image. ownerID comes from ctx, not a parameter, so a caller can't build a
// key addressing anyone else's prefix. Fixed, no extension - a re-upload
// overwrites the same object, so there's no orphan accumulation from
// repeated uploads.
func NewSwitchImageKey(ctx context.Context, switchID string) (SwitchImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return SwitchImageKey(fmt.Sprintf("switches/%s/%s/image", ownerID, switchID)), nil
}

// SwitchImageStore stores a switch's image object in a private object
// store, addressed by SwitchImageKey. Never called with the caller-facing
// presigned URL - that's minted fresh per request, never persisted.
type SwitchImageStore interface {
	// PresignGet returns a short-lived presigned GET URL for key.
	PresignGet(ctx context.Context, key SwitchImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked to
	// contentType via the Content-Type header the upload must match.
	PresignPut(ctx context.Context, key SwitchImageKey, contentType string) (url string, err error)

	// Delete removes the object at key. Idempotent: a nonexistent key is
	// not an error, matching S3's own DeleteObject semantics.
	Delete(ctx context.Context, key SwitchImageKey) error
}
