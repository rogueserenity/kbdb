package switches_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// Fixed lookup category names CreateSwitch validates open-vocabulary
// SwitchInput fields against (see internal/handlers/switches.go's
// switchMaterialCategory/switchSpringMaterialCategory/vendorCategory
// consts) - not per-spec values, since the handler always looks up these
// exact literal names. Seeded once for the whole suite (see BeforeSuite)
// rather than per-spec, so concurrent specs sharing this suite's lookup
// table rows can't race each other seeding/deleting the same category.
const (
	switchMaterialCategory       = "switch_material"
	switchSpringMaterialCategory = "switch_spring_material"
	vendorCategory               = "vendor"
)

// approvedStem/approvedSpringMaterial/approvedVendor are the one approved
// value seeded into each category above - specs testing the "approved
// value" path use these; specs testing the "unapproved value" path use any
// string not in this fixed set (e.g. "NotApproved").
const (
	approvedStem           = "POM"
	approvedSpringMaterial = "Stainless Steel"
	approvedVendor         = "Amazon"
)

func TestSwitches(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Switches Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(db.SeedLookupCategory(ctx, switchMaterialCategory, []string{approvedStem})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, switchSpringMaterialCategory, []string{approvedSpringMaterial})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, vendorCategory, []string{approvedVendor})).To(Succeed())
})

var _ = AfterSuite(func(ctx SpecContext) {
	Expect(db.DeleteLookupCategory(ctx, switchMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, switchSpringMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, vendorCategory)).To(Succeed())
})
