package mcp

import (
	"fmt"
	"time"
)

// dateLayout matches api/openapi.yaml's `format: date` and how
// openapi_types.Date parses a stored value.
const dateLayout = "2006-01-02"

// validatePurchaseDates rejects a date the REST request validator would have
// rejected at the door via `format: date`. Without this an MCP write can
// store an unparseable date, and every later REST read of that row fails in
// repoapi's date parse - a 500 with no way to repair the row through the API.
// prefix names the purchase in the error, e.g. "purchase".
func validatePurchaseDates(prefix string, orderDate, deliveryDate *string) error {
	for _, d := range []struct {
		field string
		value *string
	}{
		{prefix + ".order_date", orderDate},
		{prefix + ".delivery_date", deliveryDate},
	} {
		if d.value == nil {
			continue
		}
		if _, err := time.Parse(dateLayout, *d.value); err != nil {
			return fmt.Errorf("%s: %q must be a date in YYYY-MM-DD form", d.field, *d.value)
		}
	}

	return nil
}
