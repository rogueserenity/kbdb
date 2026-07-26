package repository

import "context"

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

// SwitchPurchase is where/how much/how many of a switch were bought.
// Simpler than Build's purchase shape — switches aren't tracked with
// order/delivery dates.
type SwitchPurchase struct {
	Vendor   *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price    *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	Quantity *int     `dynamodbav:"quantity,omitempty" json:"quantity,omitempty"`
}

// Switch is a mechanical keyboard switch in a user's collection, or shared
// with the caller. UserID is the DynamoDB partition key (the owner's
// Cognito subject); ID is the sort key. Only Brand, Name, Type, and
// Visibility are required, per api/openapi.yaml's SwitchInput schema; every
// other field (here and in SwitchMaterial/SwitchForce/SwitchSpring/
// SwitchPurchase) is a pointer so nil ("not provided") round-trips
// distinctly from an explicit zero value.
type Switch struct {
	UserID       string         `dynamodbav:"user_id" json:"-"`
	ID           string         `dynamodbav:"id" json:"id"`
	Brand        string         `dynamodbav:"brand" json:"brand"`
	Manufacturer *string        `dynamodbav:"manufacturer,omitempty" json:"manufacturer,omitempty"`
	Name         string         `dynamodbav:"name" json:"name"`
	Type         string         `dynamodbav:"type" json:"type"`
	Pins         *int           `dynamodbav:"pins,omitempty" json:"pins,omitempty"`
	FactoryLubed *bool          `dynamodbav:"factory_lubed,omitempty" json:"factory_lubed,omitempty"`
	Material     SwitchMaterial `dynamodbav:"material" json:"material"`
	Force        SwitchForce    `dynamodbav:"force" json:"force"`
	Spring       SwitchSpring   `dynamodbav:"spring" json:"spring"`
	Purchase     SwitchPurchase `dynamodbav:"purchase" json:"purchase"`
	Notes        *string        `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility   Visibility     `dynamodbav:"visibility" json:"visibility"`
}

// SwitchRepository provides access to switches. ownerID is always the
// {userId} path segment (the collection's owner, per api/openapi.yaml), not
// necessarily the caller — List returns only items whose Visibility is in
// visibilities, which the caller (a handler) derives from
// internal/authz.ReadableVisibilities before calling in.
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
	// Visibility via internal/authz.CanReadVisibility.
	Get(ctx context.Context, ownerID, id string) (*Switch, error)

	// Create stores sw, which must already have UserID and ID set (the
	// caller assigns ID - see handlers.CreateSwitch), and returns the
	// stored value. Returns ErrAlreadyExists if an item with the same
	// UserID+ID already exists, which should not happen in practice since
	// callers generate ID fresh per create, but guards against a UUID
	// collision rather than silently overwriting.
	Create(ctx context.Context, sw Switch) (*Switch, error)
}
