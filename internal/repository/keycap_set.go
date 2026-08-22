package repository

import (
	"context"
	"fmt"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

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
	KitID     string             `dynamodbav:"kit_id" json:"kit_id"`
	Name      string             `dynamodbav:"name" json:"name"`
	ImagePath *KeycapKitImageKey `dynamodbav:"image_path,omitempty" json:"image_path,omitempty"`
	Purchase  KeycapKitPurchase  `dynamodbav:"purchase" json:"purchase"`
}

// KeycapSet is a keycap set in a user's collection, or shared with the
// caller. UserID is the DynamoDB partition key (the owner's IdP-issued
// user ID); ID is the sort key. Only Brand, Name, and Visibility are
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
	// PrimaryKitID names the kit (by KitID) whose image represents this set
	// in a list/grid view. No write path sets this yet - reads must still
	// treat a dangling reference (kit deleted, or never set) as absent
	// rather than erroring.
	PrimaryKitID *string `dynamodbav:"primary_kit_id,omitempty" json:"primary_kit_id,omitempty"`
	// Version guards kit sub-mutations against a lost update when two
	// concurrent calls read-modify-write this item's Kits slice - not
	// exposed via the API, purely a repository-internal CAS mechanism.
	Version int `dynamodbav:"version" json:"-"`
}

// orderStatusProgression ranks every non-Cancelled order_status lookup
// value by how far along the purchase is - lower is less complete. Fixed
// by app semantics, not by the "order_status" lookup table's own row
// order, which isn't guaranteed to encode this meaning.
var orderStatusProgression = map[string]int{
	"Planned":   0,
	"Ordered":   1,
	"Shipped":   2,
	"Delivered": 3,
}

// AggregateOrderStatus derives a KeycapSet's overall order status from its
// kits' individual purchase.order_status values: the least-progressed
// status wins, so a set isn't shown as Delivered while a kit is still
// Ordered. "Cancelled" kits are excluded from that comparison - a
// cancelled kit doesn't drag the set backward - unless every kit is
// Cancelled, in which case the set is Cancelled too. Returns nil if kits
// is empty, or if every kit has no order_status set, or if a kit's
// order_status isn't a recognized value (unvalidated/legacy data - callers
// should not guess).
func AggregateOrderStatus(kits []KeycapKit) *string {
	var least string
	haveLeast := false
	sawCancelled := false

	for _, k := range kits {
		if k.Purchase.OrderStatus == nil {
			continue
		}
		status := *k.Purchase.OrderStatus

		if status == "Cancelled" {
			sawCancelled = true
			continue
		}

		rank, ok := orderStatusProgression[status]
		if !ok {
			return nil
		}

		if !haveLeast || rank < orderStatusProgression[least] {
			least = status
			haveLeast = true
		}
	}

	if haveLeast {
		return &least
	}

	if sawCancelled {
		cancelled := "Cancelled"
		return &cancelled
	}

	return nil
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
	// Visibility via
	// [github.com/rogueserenity/kbdb/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*KeycapSet, error)

	// Create stores ks (UserID is set from ctx, ks.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Update replaces the keycap set with ks.ID (UserID is set from ctx,
	// ks.ID must already be set to the keycap set being updated), preserving
	// Kits. Returns ErrNotFound if no keycap set with that ID exists for the
	// owner, or ErrMutationConflict if concurrent writers exhaust the retry
	// budget.
	Update(ctx context.Context, ks KeycapSet) (*KeycapSet, error)

	// Delete removes the caller's keycap set with the given id and returns
	// the KeycapKitImageKey of every kit that had an image set, so callers
	// can clean up the corresponding objects in a KeycapKitImageStore.
	// Returns ErrNotFound if id doesn't exist.
	Delete(ctx context.Context, id string) ([]KeycapKitImageKey, error)

	// AddKit appends kit to the set's Kits (kit.KitID must already be set)
	// and returns the stored kit, matching Create's shape for every other
	// entity. primary controls the set's PrimaryKitID, applied atomically
	// with the add: nil leaves it untouched, true makes the new kit
	// primary. Returns ErrNotFound if the parent set doesn't exist, or
	// ErrMutationConflict if concurrent writers exhaust the retry budget.
	AddKit(ctx context.Context, setID string, kit KeycapKit, primary *bool) (*KeycapKit, error)

	// UpdateKit returns ErrNotFound if setID or the kit doesn't exist, or
	// ErrMutationConflict if concurrent writers exhaust the retry budget.
	// primary controls the set's PrimaryKitID, applied atomically with the
	// update: nil leaves it untouched, true makes this kit primary
	// (replacing whichever kit held it before), false clears it but only
	// if this kit is the current primary.
	UpdateKit(ctx context.Context, setID string, kit KeycapKit, primary *bool) (*KeycapKit, error)

	// DeleteKit removes the kit matching kitID from setID's Kits and
	// returns the image key that was on it, or nil if it had none. If kitID
	// was the set's PrimaryKitID, that's cleared to nil too, atomically
	// with the removal - a caller never observes a set whose PrimaryKitID
	// names a kit that isn't there. Idempotent: a kitID not present in the
	// set is not an error, and returns (nil, nil). Returns ErrNotFound if
	// setID doesn't exist for the owner, or ErrMutationConflict if
	// concurrent writers exhaust the retry budget.
	DeleteKit(ctx context.Context, setID, kitID string) (*KeycapKitImageKey, error)

	// SetKitImagePath sets the kit matching kitID's ImagePath and returns
	// the updated kit. Returns ErrNotFound if setID or the kit doesn't
	// exist, or ErrMutationConflict if concurrent writers exhaust the
	// retry budget.
	SetKitImagePath(ctx context.Context, setID, kitID string, key KeycapKitImageKey) (*KeycapKit, error)

	// ClearKitImagePath clears the kit matching kitID's ImagePath and
	// returns the key that was cleared, or nil if it was already unset.
	// Idempotent: a kit with no ImagePath already set is not an error.
	// Returns ErrNotFound if setID or the kit doesn't exist, or
	// ErrMutationConflict if concurrent writers exhaust the retry budget.
	ClearKitImagePath(ctx context.Context, setID, kitID string) (*KeycapKitImageKey, error)
}

// KeycapKitImageKey is the object key a kit's image is stored under in a
// KeycapKitImageStore.
type KeycapKitImageKey string

// NewKeycapKitImageKey builds the deterministic object key for kitID's
// image within setID. ownerID comes from ctx, not a parameter, so a caller
// can't build a key addressing anyone else's prefix. Fixed, no extension -
// a re-upload overwrites the same object, so there's no orphan
// accumulation from repeated uploads.
func NewKeycapKitImageKey(ctx context.Context, setID, kitID string) (KeycapKitImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return KeycapKitImageKey(fmt.Sprintf("keycap-sets/%s/%s/kits/%s/image", ownerID, setID, kitID)), nil
}

// KeycapKitImageStore stores a kit's image object in a private object
// store, addressed by KeycapKitImageKey. Never called with the
// caller-facing presigned URL - that's minted fresh per request, never
// persisted.
type KeycapKitImageStore interface {
	// PresignGet returns a short-lived presigned GET URL for key.
	PresignGet(ctx context.Context, key KeycapKitImageKey) (url string, err error)

	// PresignPut returns a short-lived presigned PUT URL for key, locked
	// to contentType via the Content-Type header the upload must match.
	PresignPut(ctx context.Context, key KeycapKitImageKey, contentType string) (url string, err error)

	// Delete removes the object at key. Idempotent: a nonexistent key is
	// not an error, matching S3's own DeleteObject semantics.
	Delete(ctx context.Context, key KeycapKitImageKey) error

	// BestEffortDelete deletes each of keys, logging rather than returning
	// any per-key failure.
	BestEffortDelete(ctx context.Context, keys []KeycapKitImageKey)
}
