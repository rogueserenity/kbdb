package repository

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
