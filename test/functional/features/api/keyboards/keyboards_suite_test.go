package keyboards_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// Fixed lookup category names CreateKeyboard validates open-vocabulary
// KeyboardInput fields against (see internal/handlers/keyboards.go's
// validateKeyboardLookups) - not per-spec values, since the handler always
// looks up these exact literal names. Seeded once for the whole suite (see
// BeforeSuite) rather than per-spec, so concurrent specs sharing this
// suite's lookup table rows can't race each other seeding/deleting the same
// category.
const (
	keyboardSizeCategory         = "keyboard_size"
	keyboardLayoutCategory       = "keyboard_layout"
	keyboardCaseMaterialCategory = "keyboard_case_material"
	vendorCategory               = "vendor"
)

// approvedSize/approvedLayout/approvedCaseMaterial/approvedVendor are the
// one approved value seeded into each category above - specs testing the
// "approved value" path use these; specs testing the "unapproved value"
// path use any string not in this fixed set (e.g. "NotApproved").
// approvedLayout is only valid for approvedSize, per the keyboard_layout
// category's per-entry Sizes list - see validateKeyboardLayout.
const (
	approvedSize         = "60%"
	approvedLayout       = "WK"
	approvedCaseMaterial = "Aluminum"
	approvedVendor       = "Amazon"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keyboards Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(db.SeedLookupCategory(ctx, keyboardSizeCategory, []any{approvedSize})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, keyboardLayoutCategory, []any{
		map[string]any{"name": approvedLayout, "sizes": []any{approvedSize}},
	})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, keyboardCaseMaterialCategory, []any{approvedCaseMaterial})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, vendorCategory, []any{approvedVendor})).To(Succeed())
})

var _ = AfterSuite(func(ctx SpecContext) {
	Expect(db.DeleteLookupCategory(ctx, keyboardSizeCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, keyboardLayoutCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, keyboardCaseMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, vendorCategory)).To(Succeed())
})
