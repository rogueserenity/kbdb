package keycapsets_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// approvedProfile/approvedMaterial/approvedImageContentType are real values
// from internal/lookup/data/ - specs testing the "approved value" path use
// these; specs testing the "unapproved value" path use any string not in
// this fixed set (e.g. "NotApproved").
const (
	approvedProfile          = "Cherry/CYL"
	approvedMaterial         = "DyeSub PBT"
	approvedImageContentType = "image/png"
)

func TestKeycapSets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Keycap Sets Suite")
}
