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
// BeforeSuite) rather than per-spec. "vendor" is also seeded/deleted by
// switches_suite_test.go - safe because ginkgo run always executes suite
// packages one at a time (only specs within a suite are ever parallelized
// via -p/--procs), so the two BeforeSuite/AfterSuite pairs can't interleave.
const (
	keyboardSizeCategory         = "keyboard_size"
	keyboardLayoutCategory       = "keyboard_layout"
	keyboardCaseMaterialCategory = "keyboard_case_material"
	vendorCategory               = "vendor"
)

// approvedSize/approvedLayout/approvedCaseMaterial/approvedVendor are the
// approved value(s) seeded into each category above - specs testing the
// "approved value" path use these; specs testing the "unapproved value"
// path use any string not in this fixed set (e.g. "NotApproved").
// approvedLayout is only valid for approvedSize, not approvedOtherSize, per
// the keyboard_layout category's per-entry Sizes list - see
// validateKeyboardLayout. approvedOtherSize exists so a spec can send a
// size that's itself approved (passes the plain keyboard_size membership
// check) but wrong for approvedLayout, isolating the layout/size
// cross-check from the plain size check.
const (
	approvedSize         = "60%"
	approvedOtherSize    = "65%"
	approvedLayout       = "WK"
	approvedCaseMaterial = "Aluminum"
	approvedVendor       = "Amazon"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keyboards Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(db.SeedLookupCategory(ctx, keyboardSizeCategory, []any{approvedSize, approvedOtherSize})).To(Succeed())
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
