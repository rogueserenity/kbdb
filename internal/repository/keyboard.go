package repository

import (
	"context"
	"fmt"
	"sort"

	kbdbctx "github.com/rogueserenity/kbdb/internal/ctx"
)

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

// KeyboardImageEntry is one value in the Keyboard.Images map (keyed by
// image id). Seq is a repository-internal ordering key: on add it's set to
// time.Now().UnixNano(), so a new image sorts after existing ones without
// reading the current max. Wall-clock, not a monotonic source - a backward
// clock step between two adds could misorder one image (cosmetic on a
// single-user list, and self-heals if the images are ever reordered). The
// API/MCP layers sort on Seq to present images in add order; it's not in
// JSON. A future reorder endpoint would renumber entries 0..n - a later
// add's nanosecond stamp still sorts last.
type KeyboardImageEntry struct {
	Path KeyboardImageKey `dynamodbav:"path" json:"-"`
	Seq  int              `dynamodbav:"seq" json:"-"`
}

// KeyboardImage is an image id paired with its stored entry, the ordered
// element type the API/MCP layers work with. SortedKeyboardImages builds a
// []KeyboardImage from a Keyboard.Images map.
type KeyboardImage struct {
	ImageID string
	Path    KeyboardImageKey
	Seq     int
}

// SortedKeyboardImages flattens an Images map into a slice ordered by Seq
// (ascending), the order images were added in.
func SortedKeyboardImages(images map[string]KeyboardImageEntry) []KeyboardImage {
	out := make([]KeyboardImage, 0, len(images))
	for id, entry := range images {
		out = append(out, KeyboardImage{ImageID: id, Path: entry.Path, Seq: entry.Seq})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// KeyboardImagesMap builds an Images map from an ordered slice, assigning
// Seq by position. The inverse of SortedKeyboardImages, for callers holding
// images as a list (e.g. reload tooling, tests).
func KeyboardImagesMap(images []KeyboardImage) map[string]KeyboardImageEntry {
	if len(images) == 0 {
		return nil
	}
	out := make(map[string]KeyboardImageEntry, len(images))
	for i, img := range images {
		out[img.ImageID] = KeyboardImageEntry{Path: img.Path, Seq: i}
	}
	return out
}

// Keyboard is a mechanical keyboard in a user's collection, or shared with
// the caller. UserID is the DynamoDB partition key (the owner's IdP-issued
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
	// Images is keyed by image id. AddImage/DeleteImage address a single
	// entry in place (images.<id>); the API/MCP layers project it to an
	// ordered list via SortedKeyboardImages, sorting on each entry's Seq.
	Images map[string]KeyboardImageEntry `dynamodbav:"images,omitempty" json:"-"`
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

	// Delete removes the caller's keyboard with the given id. Callers clean
	// up any images it had in a KeyboardImageStore themselves, before
	// calling Delete - see
	// [github.com/rogueserenity/kbdb/internal/cascadedelete.DeleteKeyboard].
	// Idempotent: a nonexistent id is not an error.
	Delete(ctx context.Context, id string) error

	// AddImage adds image (image.ImageID must be set) to the keyboard's
	// Images map with a server-assigned Seq that sorts it after every
	// existing image (see KeyboardImageEntry.Seq); image.Seq is ignored.
	// Returns ErrNotFound if the keyboard doesn't exist, or a wrapped
	// duplicate-id error if image.ImageID is already present.
	AddImage(ctx context.Context, keyboardID string, image KeyboardImage) error

	// DeleteImage removes imageID from keyboardID's Images map and returns
	// the key that was cleared, or nil if it wasn't there. Idempotent: an
	// imageID not present is not an error. Returns ErrNotFound if keyboardID
	// doesn't exist for the owner.
	DeleteImage(ctx context.Context, keyboardID, imageID string) (*KeyboardImageKey, error)
}

// KeyboardImageKey is the object key an image is stored under in a
// KeyboardImageStore.
type KeyboardImageKey string

// NewKeyboardImageKey builds the deterministic object key for imageID's
// image within keyboardID. ownerID comes from ctx, not a parameter, so a
// caller can't build a key addressing anyone else's prefix.
func NewKeyboardImageKey(ctx context.Context, keyboardID, imageID string) (KeyboardImageKey, error) {
	ownerID, ok := kbdbctx.UserID(ctx)
	if !ok {
		return "", ErrNoUserID
	}

	return KeyboardImageKey(fmt.Sprintf("keyboards/%s/%s/images/%s", ownerID, keyboardID, imageID)), nil
}

// KeyboardImageStore is a parallel interface to [SwitchImageStore], not
// shared, since a keyboard's images are a growable array of
// server-generated ids rather than a single optional slot.
type KeyboardImageStore interface {
	// PresignGetKeyboardImage returns a short-lived presigned GET URL for
	// key.
	PresignGetKeyboardImage(ctx context.Context, key KeyboardImageKey) (url string, err error)

	// PresignPutKeyboardImage returns a short-lived presigned PUT URL for
	// key, locked to contentType via the Content-Type header the upload
	// must match.
	PresignPutKeyboardImage(ctx context.Context, key KeyboardImageKey, contentType string) (url string, err error)

	// DeleteKeyboardImage removes the object at key. Idempotent: a
	// nonexistent key is not an error, matching S3's own DeleteObject
	// semantics.
	DeleteKeyboardImage(ctx context.Context, key KeyboardImageKey) error
}
