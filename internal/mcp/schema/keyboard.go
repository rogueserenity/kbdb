package schema

// ListKeyboardsInput is the list_keyboards tool input.
type ListKeyboardsInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of keyboards to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListKeyboardsOutput is the list_keyboards tool output.
type ListKeyboardsOutput struct {
	Keyboards  []KeyboardSummary `json:"keyboards" jsonschema:"the keyboards in this page"`
	NextCursor string            `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// KeyboardSummary is the reduced keyboard shape list_keyboards returns.
type KeyboardSummary struct {
	ID          string  `json:"id" jsonschema:"the keyboard's unique id"`
	Brand       string  `json:"brand" jsonschema:"the keyboard's brand"`
	Name        string  `json:"name" jsonschema:"the keyboard's name"`
	Size        *string `json:"size,omitempty" jsonschema:"the keyboard's size, e.g. 65% or TKL"`
	Layout      *string `json:"layout,omitempty" jsonschema:"the keyboard's layout, e.g. ANSI or ISO"`
	OrderStatus *string `json:"order_status,omitempty" jsonschema:"where the order stands, for a keyboard not yet delivered"`
	HasImages   bool    `json:"has_images" jsonschema:"whether this keyboard has any images on file"`
}

// GetKeyboardInput is the get_keyboard tool input.
type GetKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the keyboard's unique id"`
	UserID     string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeyboardOutput is the get_keyboard tool output.
type GetKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the requested keyboard"`
}

// CreateKeyboardInput is the create_keyboard tool input.
type CreateKeyboardInput struct {
	KeyboardInput
}

// CreateKeyboardOutput is the create_keyboard tool output.
type CreateKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the created keyboard, including its server-generated id"`
}

// UpdateKeyboardInput is the update_keyboard tool input. Every field is
// replaced, so omitting an optional field clears it.
type UpdateKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to replace"`
	KeyboardInput
}

// UpdateKeyboardOutput is the update_keyboard tool output.
type UpdateKeyboardOutput struct {
	Keyboard Keyboard `json:"keyboard" jsonschema:"the updated keyboard"`
}

// DeleteKeyboardInput is the delete_keyboard tool input. OnDelete controls
// what happens if the keyboard is still referenced by a build: "block"
// (the default when omitted) fails the call; "cascade" deletes the
// keyboard and every referencing build; "detach" deletes the keyboard
// regardless, leaving referencing builds with a dangling keyboard_id.
type DeleteKeyboardInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard to delete"`
	OnDelete   string `json:"on_delete,omitempty" jsonschema:"how to handle a keyboard still referenced by a build: block (default), cascade, or detach"`
}

// DeleteKeyboardOutput is the delete_keyboard tool output. DeletedBuildIDs
// is populated only when on_delete was "cascade" and at least one build
// referenced the keyboard.
type DeleteKeyboardOutput struct {
	DeletedBuildIDs []string `json:"deleted_build_ids,omitempty" jsonschema:"ids of builds also deleted, when on_delete was cascade"`
}

// KeyboardInput is the writable half of a keyboard, shared by
// create_keyboard and update_keyboard.
type KeyboardInput struct {
	Brand      string            `json:"brand" jsonschema:"the keyboard's brand"`
	Name       string            `json:"name" jsonschema:"the keyboard's name"`
	Size       *string           `json:"size,omitempty" jsonschema:"the keyboard's size; must be an approved keyboard_size lookup value"`
	Layout     *string           `json:"layout,omitempty" jsonschema:"the keyboard's layout; must be an approved keyboard_layout value whose sizes include this keyboard's size"`
	Design     *KeyboardDesign   `json:"design,omitempty" jsonschema:"the case and plate makeup"`
	PCB        *KeyboardPCB      `json:"pcb,omitempty" jsonschema:"the PCB's characteristics"`
	Purchase   *KeyboardPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
	Notes      *string           `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string            `json:"visibility" jsonschema:"who can read this keyboard; one of \"public\", \"authenticated\", \"private\""`
}

// Keyboard reports HasImages rather than presigned URLs, unlike REST's
// inline Images array, to avoid handing back a URL that may have expired
// by the time an agent acts on a held result - call
// get_keyboard_image_url to fetch one on demand. Optional fields are
// pointers so a recorded zero stays distinguishable from an unset field,
// which is omitted entirely - matching what REST returns for the same
// stored keyboard.
type Keyboard struct {
	ID         string            `json:"id" jsonschema:"the keyboard's unique id"`
	Brand      string            `json:"brand" jsonschema:"the keyboard's brand"`
	Name       string            `json:"name" jsonschema:"the keyboard's name"`
	Size       *string           `json:"size,omitempty" jsonschema:"the keyboard's size, e.g. 65% or TKL"`
	Layout     *string           `json:"layout,omitempty" jsonschema:"the keyboard's layout, e.g. ANSI or ISO"`
	Design     *KeyboardDesign   `json:"design,omitempty" jsonschema:"the case and plate makeup"`
	PCB        *KeyboardPCB      `json:"pcb,omitempty" jsonschema:"the PCB's characteristics"`
	Purchase   *KeyboardPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
	Notes      *string           `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string            `json:"visibility" jsonschema:"who can read this keyboard; one of \"public\", \"authenticated\", \"private\""`
	HasImages  bool              `json:"has_images" jsonschema:"whether this keyboard has any images on file"`
}

// KeyboardDesign is a keyboard's case and plate makeup.
type KeyboardDesign struct {
	TopCase    *KeyboardMaterialColor `json:"top_case,omitempty" jsonschema:"the top case's material and color"`
	BottomCase *KeyboardMaterialColor `json:"bottom_case,omitempty" jsonschema:"the bottom case's material and color"`
	Weight     *KeyboardMaterialColor `json:"weight,omitempty" jsonschema:"the weight's material and color"`
	Plates     []string               `json:"plates,omitempty" jsonschema:"the plate materials included with the keyboard"`
}

// KeyboardMaterialColor is one physical part of a keyboard.
type KeyboardMaterialColor struct {
	Material *string `json:"material,omitempty" jsonschema:"what the part is made of"`
	Color    *string `json:"color,omitempty" jsonschema:"the part's color"`
}

// KeyboardPCB is a keyboard's PCB.
type KeyboardPCB struct {
	Thickness    *float64 `json:"thickness,omitempty" jsonschema:"PCB thickness in mm"`
	Firmware     *string  `json:"firmware,omitempty" jsonschema:"the firmware the PCB runs, e.g. QMK/VIA"`
	Assembly     *string  `json:"assembly,omitempty" jsonschema:"how the PCB is assembled, e.g. hotswap or soldered"`
	Connectivity *string  `json:"connectivity,omitempty" jsonschema:"how the PCB connects, e.g. wired or wireless"`
}

// KeyboardPurchase is a keyboard's purchase and order lifecycle. Dates
// are strings, not a date type - see
// [github.com/rogueserenity/kbdb/internal/repomcp.KeyboardToMCP].
type KeyboardPurchase struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the keyboard was bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, for one not yet delivered"`
}

// GetKeyboardImageURLInput is the get_keyboard_image_url tool's input.
type GetKeyboardImageURLInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the image belongs to"`
	ImageID    string `json:"image_id" jsonschema:"the id of the image to fetch, as returned by add_keyboard_image"`
	UserID     string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeyboardImageURLOutput is the get_keyboard_image_url tool's output.
type GetKeyboardImageURLOutput struct {
	URL string `json:"url" jsonschema:"a freshly-minted, short-lived presigned URL to fetch the image bytes from; do not cache or persist it, it expires within minutes"`
}

// AddKeyboardImageInput is the add_keyboard_image tool's input. It doesn't
// carry the image bytes themselves - see UploadURL on the output.
type AddKeyboardImageInput struct {
	KeyboardID  string `json:"keyboard_id" jsonschema:"the id of the keyboard to add an image to"`
	ContentType string `json:"content_type" jsonschema:"the image's MIME type; must be an approved image_content_type lookup value"`
}

// AddKeyboardImageOutput is the add_keyboard_image tool's output. UploadURL
// is a presigned S3 PUT URL - the caller uploads the image bytes directly
// to it, matching REST's AddKeyboardImage; the tool call itself never
// carries image bytes.
type AddKeyboardImageOutput struct {
	ImageID   string `json:"image_id" jsonschema:"the newly-created image's id"`
	UploadURL string `json:"upload_url" jsonschema:"a freshly-minted, short-lived presigned URL to PUT the image bytes to directly, using the requested content_type as the Content-Type header; do not cache or persist it, it expires within minutes"`
}

// DeleteKeyboardImageInput is the delete_keyboard_image tool's input.
type DeleteKeyboardImageInput struct {
	KeyboardID string `json:"keyboard_id" jsonschema:"the id of the keyboard the image belongs to"`
	ImageID    string `json:"image_id" jsonschema:"the id of the image to remove, as returned by add_keyboard_image"`
}

// DeleteKeyboardImageOutput is the delete_keyboard_image tool's output.
// Deleting is idempotent, so there is no payload.
type DeleteKeyboardImageOutput struct{}
