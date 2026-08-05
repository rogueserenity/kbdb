package switches_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// The lookup categories create_switch/update_switch validate
// open-vocabulary fields against, seeded once for the whole suite. The
// deployed lookup table is empty in the functional environment, so a spec
// exercising the approved-value path has to put the values there itself -
// same approach as the REST switches suite, which seeds these identical
// categories. Safe despite the overlap because ginkgo runs suite packages
// one at a time; only specs within a suite are ever parallelized.
const (
	switchTypeCategory           = "switch_type"
	switchMaterialCategory       = "switch_material"
	switchSpringMaterialCategory = "switch_spring_material"
	vendorCategory               = "vendor"
)

// The one approved value seeded into each category above. Specs testing the
// unapproved-value path use any string outside this set.
const (
	approvedType           = "Linear"
	approvedStem           = "POM"
	approvedSpringMaterial = "Stainless Steel"
	approvedVendor         = "Amazon"
)

func TestSwitches(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Switches Suite")
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(db.SeedLookupCategory(ctx, switchTypeCategory, []any{approvedType})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, switchMaterialCategory, []any{approvedStem})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, switchSpringMaterialCategory, []any{approvedSpringMaterial})).To(Succeed())
	Expect(db.SeedLookupCategory(ctx, vendorCategory, []any{approvedVendor})).To(Succeed())
})

var _ = AfterSuite(func(ctx SpecContext) {
	Expect(db.DeleteLookupCategory(ctx, switchTypeCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, switchMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, switchSpringMaterialCategory)).To(Succeed())
	Expect(db.DeleteLookupCategory(ctx, vendorCategory)).To(Succeed())
})

type getOutput struct {
	Switch struct {
		ID         string `json:"id"`
		Brand      string `json:"brand"`
		Name       string `json:"name"`
		Type       string `json:"type"`
		Visibility string `json:"visibility"`
	} `json:"switch"`
}

func decodeGetOutput(result *sdkmcp.CallToolResult) getOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out getOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}
