package lookups_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a lookup category", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		category string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		category = "functional-test-category-" + uuid.NewString()

		token, err := api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteLookupCategory(ctx, category)).To(Succeed())
	})

	Context("given the category exists", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("the get_lookup tool is called with that category", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_lookup", map[string]any{"category": category})
			})

			It("succeeds and returns the category's values", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())

				var out struct {
					Category string   `json:"category"`
					Values   []string `json:"values"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Category).To(Equal(category))
				Expect(out.Values).To(Equal([]string{"a", "b"}))
			})
		})
	})

	Context("given the category does not exist", func() {
		When("the get_lookup tool is called with that category", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_lookup", map[string]any{"category": "functional-test-category-missing-" + uuid.NewString()})
			})

			It("returns an MCP tool error result, not a transport failure", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
