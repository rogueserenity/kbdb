package keyboards_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a keyboard over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		ownerID    string
		keyboardID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		keyboardID = "functional-test-keyboard-" + uuid.NewString()
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client, ownerID = api.NewAuthenticatedMCPClient(ctx)
		})

		Context("given the caller owns the keyboard", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
			})

			When("the get_keyboard tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keyboard", map[string]any{"keyboard_id": keyboardID})
				})

				It("defaults to the caller's own collection and returns the keyboard", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Keyboard.ID).To(Equal(keyboardID))
					Expect(out.Keyboard.Brand).To(Equal("Keychron"))
					Expect(out.Keyboard.Visibility).To(Equal("private"))

					By("round-tripping the nested groups out of DynamoDB")
					Expect(out.Keyboard.Design).NotTo(BeNil())
					Expect(out.Keyboard.Design.TopCase).NotTo(BeNil())
					Expect(*out.Keyboard.Design.TopCase.Material).To(Equal("Aluminum"))
					Expect(out.Keyboard.Design.Plates).To(Equal([]string{"Brass"}))
					Expect(out.Keyboard.PCB).NotTo(BeNil())
					Expect(*out.Keyboard.PCB.Firmware).To(Equal("QMK/VIA"))
					Expect(out.Keyboard.Purchase).NotTo(BeNil())
					Expect(*out.Keyboard.Purchase.Vendor).To(Equal("Amazon"))
					Expect(out.Keyboard.Purchase.Price).NotTo(BeNil())
					Expect(*out.Keyboard.Purchase.Price).To(Equal(329.99))

					By("omitting a group whose fields are all unset")
					Expect(out.Keyboard.Design.BottomCase).To(BeNil())
				})
			})
		})

		Context("given the keyboard never existed", func() {
			When("the get_keyboard tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keyboard", map[string]any{"keyboard_id": keyboardID})
				})

				It("returns an MCP tool error result, not a transport failure", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a private keyboard", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				Expect(db.SeedKeyboard(ctx, otherID, keyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, keyboardID)).To(Succeed())
			})

			When("the get_keyboard tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keyboard", map[string]any{
						"keyboard_id": keyboardID,
						"user_id":     otherID,
					})
				})

				It("is indistinguishable from the keyboard not existing", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a public keyboard", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherID = api.NewOtherUserID(ctx)

				Expect(db.SeedKeyboard(ctx, otherID, keyboardID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, keyboardID)).To(Succeed())
			})

			When("the get_keyboard tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keyboard", map[string]any{
						"keyboard_id": keyboardID,
						"user_id":     otherID,
					})
				})

				It("returns the keyboard with purchase.price omitted", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetOutput(result)
					Expect(out.Keyboard.ID).To(Equal(keyboardID))

					By("still including non-price purchase fields")
					Expect(out.Keyboard.Purchase).NotTo(BeNil())
					Expect(*out.Keyboard.Purchase.Vendor).To(Equal("Amazon"))

					By("omitting price")
					Expect(out.Keyboard.Purchase.Price).To(BeNil())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the get_keyboard tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_keyboard", map[string]any{"keyboard_id": keyboardID})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
