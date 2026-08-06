package keycapsets_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// Fixed lookup category names CreateKeycapSet/SetKeycapKitImage validate
// open-vocabulary fields against (see
// internal/handlers/keycap_sets.go's validateKeycapSetLookups and
// SetKeycapKitImage) - not per-spec values, since the handlers always look
// up these exact literal names. Seeded once for the whole suite (see
// BeforeSuite) rather than per-spec.
const (
	keycapProfileCategory    = "keycap_profile"
	keycapMaterialCategory   = "keycap_material"
	imageContentTypeCategory = "image_content_type"
)

// approvedProfile/approvedMaterial/approvedImageContentType are the
// approved values seeded into each category above - specs testing the
// "approved value" path use these; specs testing the "unapproved value"
// path use any string not in this fixed set (e.g. "NotApproved").
const (
	approvedProfile          = "Cherry"
	approvedMaterial         = "ABS"
	approvedImageContentType = "image/png"
)

func TestKeycapSets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "REST Keycap Sets Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(db.SeedLookupCategory(ctx, keycapProfileCategory, []any{approvedProfile})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, keycapMaterialCategory, []any{approvedMaterial})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, imageContentTypeCategory, []any{approvedImageContentType})).To(Succeed())
})

var _ = AfterSuite(func(ctx SpecContext) {
	Expect(db.DeleteLookupCategory(ctx, keycapProfileCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, keycapMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, imageContentTypeCategory)).To(Succeed())
})
