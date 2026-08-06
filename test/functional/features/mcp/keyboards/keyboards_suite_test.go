package keyboards_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Keyboards Suite")
}

// The lookup categories create_keyboard/update_keyboard validate against,
// seeded once for the whole suite - the functional lookup table is empty
// otherwise. approvedLayout is valid only for approvedSize, so
// approvedOtherSize lets a spec send a size that passes the plain
// keyboard_size check but is wrong for the layout, isolating the
// cross-field rule from the plain membership check.
const (
	keyboardSizeCategory         = "keyboard_size"
	keyboardLayoutCategory       = "keyboard_layout"
	keyboardCaseMaterialCategory = "keyboard_case_material"
	vendorCategory               = "vendor"
)

const (
	approvedSize         = "60%"
	approvedOtherSize    = "65%"
	approvedLayout       = "WK"
	approvedCaseMaterial = "Aluminum"
	approvedVendor       = "Amazon"
)

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

type getOutput struct {
	Keyboard struct {
		ID         string  `json:"id"`
		Brand      string  `json:"brand"`
		Name       string  `json:"name"`
		Size       *string `json:"size"`
		Visibility string  `json:"visibility"`
		Design     *struct {
			TopCase *struct {
				Material *string `json:"material"`
				Color    *string `json:"color"`
			} `json:"top_case"`
			BottomCase *struct{} `json:"bottom_case"`
			Plates     []string  `json:"plates"`
		} `json:"design"`
		PCB *struct {
			Firmware *string `json:"firmware"`
		} `json:"pcb"`
		Purchase *struct {
			Vendor      *string `json:"vendor"`
			OrderStatus *string `json:"order_status"`
		} `json:"purchase"`
	} `json:"keyboard"`
}

func decodeGetOutput(result *sdkmcp.CallToolResult) getOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out getOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

type listKeyboard struct {
	ID          string  `json:"id"`
	Brand       string  `json:"brand"`
	Name        string  `json:"name"`
	OrderStatus *string `json:"order_status"`
}

type listOutput struct {
	Keyboards  []listKeyboard `json:"keyboards"`
	NextCursor string         `json:"next_cursor"`
}

func decodeListOutput(result *sdkmcp.CallToolResult) listOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out listOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

func idsOf(out listOutput) []string {
	ids := make([]string, 0, len(out.Keyboards))
	for _, kb := range out.Keyboards {
		ids = append(ids, kb.ID)
	}

	return ids
}

func seededBy(out listOutput, id string) *listKeyboard {
	for i := range out.Keyboards {
		if out.Keyboards[i].ID == id {
			return &out.Keyboards[i]
		}
	}

	return nil
}
