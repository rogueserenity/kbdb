package repoapi

import (
	"fmt"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// dateLayout matches how openapi_types.Date marshals/unmarshals.
const dateLayout = "2006-01-02"

func parseAPIDate(s string) (*openapi_types.Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("stored date %q does not match layout %q: %w", s, dateLayout, err)
	}

	return &openapi_types.Date{Time: t}, nil
}

// sumKnownCosts sums the non-nil components, treating a nil one as
// excluded rather than zero. Returns nil if none are set.
func sumKnownCosts(components ...*float64) *float64 {
	var total float64
	var haveAny bool
	for _, c := range components {
		if c == nil {
			continue
		}
		total += *c
		haveAny = true
	}
	if !haveAny {
		return nil //nolint:nilnil // no known-priced components is a valid, expected result
	}

	return &total
}
