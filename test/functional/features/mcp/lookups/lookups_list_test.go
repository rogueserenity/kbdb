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

var _ = Describe("Listing lookup categories", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		category string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		category = "functional-test-category-" + uuid.NewString()
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteLookupCategory(ctx, category)).To(Succeed())
	})

	Context("given a valid bearer token and the lookup table has a category", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())
			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)

			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("succeeds and includes the seeded category", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())

				raw, err := json.Marshal(result.StructuredContent)
				Expect(err).NotTo(HaveOccurred())

				var out struct {
					Categories []string `json:"categories"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.Categories).To(ContainElement(category))
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_lookups tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
