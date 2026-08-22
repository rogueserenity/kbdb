package keyboards_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// approvedSize/approvedLayout/approvedCaseMaterial/approvedVendor are real
// values from internal/lookup/data/. approvedLayout is only valid for
// approvedSize, not approvedOtherSize - approvedOtherSize lets a spec send
// a size that passes the plain keyboard_size check but is wrong for
// approvedLayout, isolating the layout/size cross-check.
const (
	approvedSize             = "60%"
	approvedOtherSize        = "40%"
	approvedLayout           = "WK"
	approvedCaseMaterial     = "Aluminum"
	approvedVendor           = "Amazon"
	approvedImageContentType = "image/png"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Keyboards Suite")
}
