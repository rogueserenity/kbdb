package keyboards_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// approvedSize/approvedLayout/approvedCaseMaterial/approvedVendor are real
// values from internal/lookup/data/ - specs testing the "approved value"
// path use these; specs testing the "unapproved value" path use any string
// not in this fixed set (e.g. "NotApproved").
// approvedLayout is only valid for approvedSize, not approvedOtherSize, per
// the keyboard_layout category's per-entry Sizes list - see
// validateKeyboardLayout. approvedOtherSize exists so a spec can send a
// size that's itself approved (passes the plain keyboard_size membership
// check) but wrong for approvedLayout, isolating the layout/size
// cross-check from the plain size check.
const (
	approvedSize         = "60%"
	approvedOtherSize    = "40%"
	approvedLayout       = "WK"
	approvedCaseMaterial = "Aluminum"
	approvedVendor       = "Amazon"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Keyboards Suite")
}
