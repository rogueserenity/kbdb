package keyboards_test

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Creating a keyboard over MCP", func() {
	var (
		client    *api.MCPClient
		result    *sdkmcp.CallToolResult
		err       error
		ownerID   string
		createdID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		createdID = ""
	})

	captureCreatedID := func(r *sdkmcp.CallToolResult) {
		if r == nil || r.IsError {
			return
		}
		createdID = decodeGetOutput(r).Keyboard.ID
	}

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteKeyboard(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given valid arguments", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       approvedSize,
						"layout":     approvedLayout,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("creates the keyboard in the caller's own collection", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Keyboard.ID).NotTo(BeEmpty(), "the server assigns the id")
					Expect(out.Keyboard.Brand).To(Equal("Mode"))
					Expect(out.Keyboard.Visibility).NotTo(BeNil())
					Expect(*out.Keyboard.Visibility).To(Equal("private"))
				})
			})
		})

		Context("given plates and PCBs, one PCB with its own purchase", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"visibility": "private",
						"purchase":   map[string]any{"price": 300},
						"plates":     []any{map[string]any{"material": "AL"}},
						"pcbs": []any{
							map[string]any{"firmware": "QMK/VIA"},
							map[string]any{"firmware": "ZMK", "purchase": map[string]any{"price": 45, "order_date": "2026-03-01"}},
						},
					})
					captureCreatedID(result)
				})

				It("stores every part with a server-generated id and prices the extra PCB", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Keyboard.Plates).To(HaveLen(1))
					Expect(out.Keyboard.Plates[0].ID).NotTo(BeEmpty())
					Expect(out.Keyboard.Plates[0].Material).To(Equal("AL"))
					Expect(out.Keyboard.PCBs).To(HaveLen(2))
					Expect(out.Keyboard.PCBs[0].ID).NotTo(BeEmpty())
					Expect(out.Keyboard.PCBs[1].ID).NotTo(BeEmpty())
					Expect(out.Keyboard.PCBs[0].ID).NotTo(Equal(out.Keyboard.PCBs[1].ID))
					Expect(*out.Keyboard.PCBs[1].Firmware).To(Equal("ZMK"))
					Expect(out.Keyboard.TotalCost).NotTo(BeNil())
					Expect(*out.Keyboard.TotalCost).To(Equal(345.0))
				})
			})
		})

		Context("given a size that is not an approved lookup value", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       "NotApproved",
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		// Both values are individually approved; only the pairing is wrong.
		Context("given a size that is approved but not valid for the layout", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"size":       approvedOtherSize,
						"layout":     approvedLayout,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		// REST rejects this at the door via openapi.yaml's `format: date`;
		// MCP has no such validator, and a stored bad date makes every later
		// REST read of the row fail its date parse.
		Context("given a malformed purchase date", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"visibility": "private",
						"purchase":   map[string]any{"order_date": "next tuesday"},
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given an invalid visibility", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "Mode",
						"name":       "Sixty",
						"visibility": "everyone",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given a required field is present but blank", func() {
			When("the create_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
						"brand":      "",
						"name":       "Sixty",
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the create_keyboard tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_keyboard", map[string]any{
					"brand":      "Mode",
					"name":       "Sixty",
					"visibility": "private",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
