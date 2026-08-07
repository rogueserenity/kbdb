package schema

// ListKeycapSetsInput is the list_keycap_sets tool input.
type ListKeycapSetsInput struct {
	UserID string `json:"user_id,omitempty" jsonschema:"whose collection to list; omit for your own"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of keycap sets to return (1-100, default 20)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"resume from a previous call's next_cursor"`
}

// ListKeycapSetsOutput is the list_keycap_sets tool output.
type ListKeycapSetsOutput struct {
	KeycapSets []KeycapSetSummary `json:"keycap_sets" jsonschema:"the keycap sets in this page"`
	NextCursor string             `json:"next_cursor,omitempty" jsonschema:"pass as cursor to fetch the next page; empty when there are no more"`
}

// KeycapSetSummary is the reduced keycap set shape list_keycap_sets returns.
type KeycapSetSummary struct {
	ID      string  `json:"id" jsonschema:"the keycap set's unique id"`
	Brand   string  `json:"brand" jsonschema:"the keycap set's brand"`
	Name    string  `json:"name" jsonschema:"the keycap set's name"`
	Profile *string `json:"profile,omitempty" jsonschema:"the keycap set's profile, e.g. Cherry or OEM"`
}

// GetKeycapSetInput is the get_keycap_set tool input.
type GetKeycapSetInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the keycap set's unique id"`
	UserID      string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeycapSetOutput is the get_keycap_set tool output.
type GetKeycapSetOutput struct {
	KeycapSet KeycapSet `json:"keycap_set" jsonschema:"the requested keycap set"`
}

// KeycapSet is the full keycap set shape, including its kits.
type KeycapSet struct {
	ID         string      `json:"id" jsonschema:"the keycap set's unique id"`
	Brand      string      `json:"brand" jsonschema:"the keycap set's brand"`
	Name       string      `json:"name" jsonschema:"the keycap set's name"`
	Profile    *string     `json:"profile,omitempty" jsonschema:"the keycap set's profile, e.g. Cherry or OEM"`
	Material   *string     `json:"material,omitempty" jsonschema:"what the keycaps are made of"`
	Notes      *string     `json:"notes,omitempty" jsonschema:"free-form notes"`
	Visibility string      `json:"visibility" jsonschema:"who can read this keycap set; one of \"public\", \"authenticated\", \"private\""`
	Kits       []KeycapKit `json:"kits,omitempty" jsonschema:"the kits purchased as part of this set"`
}

// KeycapKit is one purchase within a keycap set. HasImage reports whether an
// image is on file, without minting a URL every set/kit read would then have
// to carry - call get_keycap_kit_image_url on demand instead. This is a
// deliberate divergence from REST's KeycapKitImage.Url: a presigned URL
// handed back inline would be short-lived and an agent may hold this result
// across turns, long after the URL has expired.
type KeycapKit struct {
	KitID    string            `json:"kit_id" jsonschema:"the kit's unique id, scoped to its parent keycap set"`
	Name     string            `json:"name" jsonschema:"the kit's name, e.g. Base or Novelties"`
	HasImage bool              `json:"has_image" jsonschema:"whether an image is on file for this kit; call get_keycap_kit_image_url to fetch it"`
	Purchase *KeycapKitPurchase `json:"purchase,omitempty" jsonschema:"where it was bought and the order's status"`
}

// KeycapKitPurchase is a kit's purchase and order lifecycle, independent of
// the parent set's - a set can be assembled from kits bought separately.
// Dates are strings, not a date type - see repomcp.keycapKitPurchaseToMCP.
type KeycapKitPurchase struct {
	Vendor       *string  `json:"vendor,omitempty" jsonschema:"where the kit was bought"`
	Price        *float64 `json:"price,omitempty" jsonschema:"price paid"`
	OrderDate    *string  `json:"order_date,omitempty" jsonschema:"when it was ordered (YYYY-MM-DD)"`
	DeliveryDate *string  `json:"delivery_date,omitempty" jsonschema:"when it arrived (YYYY-MM-DD)"`
	OrderStatus  *string  `json:"order_status,omitempty" jsonschema:"where the order stands, for one not yet delivered"`
}

// GetKeycapKitImageURLInput is the get_keycap_kit_image_url tool input.
type GetKeycapKitImageURLInput struct {
	KeycapSetID string `json:"keycap_set_id" jsonschema:"the id of the keycap set the kit belongs to"`
	KitID       string `json:"kit_id" jsonschema:"the id of the kit to fetch the image for"`
	UserID      string `json:"user_id,omitempty" jsonschema:"whose collection to read from; omit for your own"`
}

// GetKeycapKitImageURLOutput is the get_keycap_kit_image_url tool output.
type GetKeycapKitImageURLOutput struct {
	URL string `json:"url" jsonschema:"a freshly-minted, short-lived presigned URL to fetch the image; do not cache or persist it, it expires within minutes"`
}
