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

// Keyboard is the full keyboard shape. Optional fields are pointers so a
// recorded zero stays distinguishable from an unset field, which is omitted
// entirely - matching what REST returns for the same stored keyboard.
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
	Visibility string            `json:"visibility" jsonschema:"who can read this keyboard: public, authenticated, or private"`
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
// are strings, not a date type - see repomcp.keyboardPurchaseToMCP.
type KeyboardPurchase struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the keyboard was bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, for one not yet delivered"`
}
