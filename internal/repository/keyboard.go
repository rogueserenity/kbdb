package repository

import "context"

// KeyboardMaterialColor is a physical part described by material and color
// (top case, bottom case, or weight).
type KeyboardMaterialColor struct {
	Material *string `dynamodbav:"material,omitempty" json:"material,omitempty"`
	Color    *string `dynamodbav:"color,omitempty" json:"color,omitempty"`
}

// KeyboardDesign is a keyboard's physical case/plate makeup.
type KeyboardDesign struct {
	TopCase    KeyboardMaterialColor `dynamodbav:"top_case" json:"top_case"`
	BottomCase KeyboardMaterialColor `dynamodbav:"bottom_case" json:"bottom_case"`
	Weight     KeyboardMaterialColor `dynamodbav:"weight" json:"weight"`
	Plates     []string              `dynamodbav:"plates,omitempty" json:"plates,omitempty"`
}

// KeyboardPCB is a keyboard's PCB characteristics.
type KeyboardPCB struct {
	Thickness    *float64 `dynamodbav:"thickness,omitempty" json:"thickness,omitempty"`
	Firmware     *string  `dynamodbav:"firmware,omitempty" json:"firmware,omitempty"`
	Assembly     *string  `dynamodbav:"assembly,omitempty" json:"assembly,omitempty"`
	Connectivity *string  `dynamodbav:"connectivity,omitempty" json:"connectivity,omitempty"`
}

// KeyboardPurchase is where/how much a keyboard was bought, plus its
// order lifecycle. Fuller shape than SwitchPurchase - keyboards are
// tracked with order/delivery dates and status.
type KeyboardPurchase struct {
	Vendor       *string  `dynamodbav:"vendor,omitempty" json:"vendor,omitempty"`
	Price        *float64 `dynamodbav:"price,omitempty" json:"price,omitempty"`
	OrderDate    *string  `dynamodbav:"order_date,omitempty" json:"order_date,omitempty"`
	DeliveryDate *string  `dynamodbav:"delivery_date,omitempty" json:"delivery_date,omitempty"`
	OrderStatus  *string  `dynamodbav:"order_status,omitempty" json:"order_status,omitempty"`
}

// Keyboard is a mechanical keyboard in a user's collection, or shared with
// the caller. UserID is the DynamoDB partition key (the owner's WorkOS
// user ID); ID is the sort key. Only Brand, Name, and Visibility are
// required, per api/openapi.yaml's KeyboardInput schema; every other field
// (here and in KeyboardMaterialColor/KeyboardDesign/KeyboardPCB/
// KeyboardPurchase) is a pointer so nil ("not provided") round-trips
// distinctly from an explicit zero value.
type Keyboard struct {
	UserID     string           `dynamodbav:"user_id" json:"-"`
	ID         string           `dynamodbav:"id" json:"id"`
	Brand      string           `dynamodbav:"brand" json:"brand"`
	Name       string           `dynamodbav:"name" json:"name"`
	Size       *string          `dynamodbav:"size,omitempty" json:"size,omitempty"`
	Layout     *string          `dynamodbav:"layout,omitempty" json:"layout,omitempty"`
	Design     KeyboardDesign   `dynamodbav:"design" json:"design"`
	PCB        KeyboardPCB      `dynamodbav:"pcb" json:"pcb"`
	Purchase   KeyboardPurchase `dynamodbav:"purchase" json:"purchase"`
	Notes      *string          `dynamodbav:"notes,omitempty" json:"notes,omitempty"`
	Visibility Visibility       `dynamodbav:"visibility" json:"visibility"`
}

// KeyboardRepository provides access to keyboards.
type KeyboardRepository interface {
	// List returns up to limit keyboards owned by ownerID whose Visibility
	// is in visibilities, ordered by ID. cursor, if non-empty, resumes from
	// a previous call's returned cursor; the returned cursor is empty when
	// there are no more pages.
	List(ctx context.Context, ownerID string, visibilities []Visibility, limit int, cursor string) (keyboards []Keyboard, nextCursor string, err error)

	// Get returns the keyboard owned by ownerID with the given id, or
	// ErrNotFound if it doesn't exist. Get doesn't take a visibility
	// argument: unlike List, it fetches by exact key regardless of
	// visibility - the caller (a handler) checks the returned item's
	// Visibility via
	// [github.com/rogueserenity/kbdb/internal/authz.CanReadVisibility].
	Get(ctx context.Context, ownerID, id string) (*Keyboard, error)

	// Create stores kb (UserID is set from ctx, kb.ID must already be set).
	// Returns ErrAlreadyExists on an ID collision.
	Create(ctx context.Context, kb Keyboard) (*Keyboard, error)

	// Update replaces the caller's keyboard (UserID is set from ctx, kb.ID
	// must already be set to the keyboard being updated). Returns
	// ErrNotFound if no keyboard with that id exists for the caller.
	Update(ctx context.Context, kb Keyboard) (*Keyboard, error)

	// Delete removes the caller's keyboard with the given id. Idempotent: a
	// nonexistent id is not an error.
	Delete(ctx context.Context, id string) error
}
