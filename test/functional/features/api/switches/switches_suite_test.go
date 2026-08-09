package switches_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// approvedType/approvedStem/approvedSpringMaterial/approvedVendor are real
// values from internal/lookup/data/ - specs testing the "approved value"
// path use these; specs testing the "unapproved value" path use any string
// not in the seeded set (e.g. "NotApproved").
const (
	approvedType           = "Linear"
	approvedStem           = "POM"
	approvedSpringMaterial = "Stainless Steel"
	approvedVendor         = "Amazon"
)

func TestSwitches(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Switches Suite")
}
