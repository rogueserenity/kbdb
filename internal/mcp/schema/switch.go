package schema

// ListSwitchesInput is the list_switches tool arguments.
type ListSwitchesInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of switches to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListSwitchesOutput is the list_switches tool result.
type ListSwitchesOutput struct {
	Switches   []SwitchSummary `json:"switches" jsonschema:"the switches in this page"`
	NextCursor string          `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// SwitchSummary is the abbreviated switch shape returned by list_switches.
// Call get_switch for the remaining fields.
type SwitchSummary struct {
	ID    string `json:"id" jsonschema:"the switch's unique id"`
	Brand string `json:"brand" jsonschema:"the switch's brand"`
	Name  string `json:"name" jsonschema:"the switch's name"`
	Type  string `json:"type" jsonschema:"the switch type, e.g. linear or tactile"`
}

// GetSwitchInput is the get_switch tool arguments.
type GetSwitchInput struct {
	SwitchID string `json:"switch_id" jsonschema:"the switch's unique id"`
	UserID   string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetSwitchOutput is the get_switch tool result.
type GetSwitchOutput struct {
	Switch Switch `json:"switch" jsonschema:"the requested switch"`
}

// Switch is the full switch shape. Optional fields are pointers so a
// recorded zero (a free purchase, a 0g force) stays distinguishable from an
// unset field, which is omitted entirely - matching what REST returns for
// the same stored switch.
type Switch struct {
	ID           string          `json:"id" jsonschema:"the switch's unique id"`
	Brand        string          `json:"brand" jsonschema:"the switch's brand"`
	Manufacturer *string         `json:"manufacturer,omitempty" jsonschema:"who physically manufactures the switch, if different from the brand"`
	Name         string          `json:"name" jsonschema:"the switch's name"`
	Type         string          `json:"type" jsonschema:"the switch type, e.g. linear or tactile"`
	Pins         *int            `json:"pins,omitempty" jsonschema:"pin count, typically 3 or 5"`
	FactoryLubed *bool           `json:"factory_lubed,omitempty" jsonschema:"whether the switch ships pre-lubed"`
	Material     *SwitchMaterial `json:"material,omitempty" jsonschema:"housing and stem materials"`
	Force        *SwitchForce    `json:"force,omitempty" jsonschema:"actuation and bottom-out force, in grams"`
	Spring       *SwitchSpring   `json:"spring,omitempty" jsonschema:"spring material and travel distances, in mm"`
	Purchase     *SwitchPurchase `json:"purchase,omitempty" jsonschema:"where and how many were bought"`
	Notes        *string         `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility   string          `json:"visibility" jsonschema:"who can read this switch: public, authenticated, or private"`
}

// SwitchMaterial is a switch's housing and stem materials.
type SwitchMaterial struct {
	TopHousing    *string `json:"top_housing,omitempty" jsonschema:"top housing material"`
	BottomHousing *string `json:"bottom_housing,omitempty" jsonschema:"bottom housing material"`
	Stem          *string `json:"stem,omitempty" jsonschema:"stem material"`
}

// SwitchForce is a switch's nominal actuation and bottom-out force, in grams.
type SwitchForce struct {
	Actuation *float64 `json:"actuation,omitempty" jsonschema:"actuation force in grams"`
	BottomOut *float64 `json:"bottom_out,omitempty" jsonschema:"bottom-out force in grams"`
}

// SwitchSpring is a switch's spring material and travel distances, in mm.
type SwitchSpring struct {
	Material    *string  `json:"material,omitempty" jsonschema:"spring material"`
	PreTravel   *float64 `json:"pre_travel,omitempty" jsonschema:"pre-travel distance in mm"`
	TotalTravel *float64 `json:"total_travel,omitempty" jsonschema:"total travel distance in mm"`
}

// SwitchPurchase is where, for how much, and how many of a switch were bought.
type SwitchPurchase struct {
	Vendor   *string  `json:"vendor,omitempty" jsonschema:"where the switches were bought"`
	Price    *float64 `json:"price,omitempty" jsonschema:"price paid"`
	Quantity *int     `json:"quantity,omitempty" jsonschema:"how many were bought"`
}
