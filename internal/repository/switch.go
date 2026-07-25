package repository

import "context"

// SwitchMaterial is the housing/stem material makeup of a switch.
type SwitchMaterial struct {
	TopHousing    string `dynamodbav:"top_housing" json:"top_housing"`
	BottomHousing string `dynamodbav:"bottom_housing" json:"bottom_housing"`
	Stem          string `dynamodbav:"stem" json:"stem"`
}

// SwitchForce is a switch's nominal actuation/bottom-out force, in grams.
type SwitchForce struct {
	Actuation float64 `dynamodbav:"actuation" json:"actuation"`
	BottomOut float64 `dynamodbav:"bottom_out" json:"bottom_out"`
}

// SwitchSpring is a switch's spring material and travel distances (mm).
type SwitchSpring struct {
	Material    string  `dynamodbav:"material" json:"material"`
	PreTravel   float64 `dynamodbav:"pre_travel" json:"pre_travel"`
	TotalTravel float64 `dynamodbav:"total_travel" json:"total_travel"`
}

// SwitchPurchase is where/how much/how many of a switch were bought.
// Simpler than Build's purchase shape — switches aren't tracked with
// order/delivery dates.
type SwitchPurchase struct {
	Vendor   string  `dynamodbav:"vendor" json:"vendor"`
	Price    float64 `dynamodbav:"price" json:"price"`
	Quantity int     `dynamodbav:"quantity" json:"quantity"`
}

// Switch is a mechanical keyboard switch in a user's collection, or shared
// with the caller. UserID is the DynamoDB partition key (the owner's
// Cognito subject); ID is the sort key.
type Switch struct {
	UserID       string         `dynamodbav:"user_id" json:"-"`
	ID           string         `dynamodbav:"id" json:"id"`
	Brand        string         `dynamodbav:"brand" json:"brand"`
	Manufacturer string         `dynamodbav:"manufacturer" json:"manufacturer"`
	Name         string         `dynamodbav:"name" json:"name"`
	Type         string         `dynamodbav:"type" json:"type"`
	Pins         int            `dynamodbav:"pins" json:"pins"`
	FactoryLubed bool           `dynamodbav:"factory_lubed" json:"factory_lubed"`
	Material     SwitchMaterial `dynamodbav:"material" json:"material"`
	Force        SwitchForce    `dynamodbav:"force" json:"force"`
	Spring       SwitchSpring   `dynamodbav:"spring" json:"spring"`
	Purchase     SwitchPurchase `dynamodbav:"purchase" json:"purchase"`
	Notes        string         `dynamodbav:"notes" json:"notes"`
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
