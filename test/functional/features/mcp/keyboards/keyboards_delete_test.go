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

var _ = Describe("Deleting a keyboard over MCP", func() {
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
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given the caller owns the keyboard", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
			})

			When("the delete_keyboard tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keyboard", map[string]any{
						"keyboard_id": keyboardID,
					})
				})

				It("removes the keyboard", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					By("a subsequent get_keyboard no longer finding it")
					check, checkErr := client.CallTool(ctx, "get_keyboard", map[string]any{
						"keyboard_id": keyboardID,
					})
					Expect(checkErr).NotTo(HaveOccurred())
					Expect(check.IsError).To(BeTrue())
				})
			})
		})

		Context("given the keyboard never existed", func() {
			When("the delete_keyboard tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keyboard", map[string]any{
						"keyboard_id": keyboardID,
					})
				})

				It("succeeds, since deleting is idempotent", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
				})
			})
		})

		Context("given another user owns the keyboard", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				Expect(db.SeedKeyboard(ctx, otherID, keyboardID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeyboard(ctx, otherID, keyboardID)).To(Succeed())
			})

			When("the delete_keyboard tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keyboard", map[string]any{
						"keyboard_id": keyboardID,
					})
				})

				It("leaves the other user's keyboard in place", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					By("the other user still being able to read it")
					otherToken, tokenErr := api.SecondUserAuthToken(ctx)
					Expect(tokenErr).NotTo(HaveOccurred())
					otherClient := api.NewMCPClient(support.BaseURL()+"/mcp", otherToken)

					check, checkErr := otherClient.CallTool(ctx, "get_keyboard", map[string]any{
						"keyboard_id": keyboardID,
					})
					Expect(checkErr).NotTo(HaveOccurred())
					Expect(check.IsError).To(BeFalse())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the delete_keyboard tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_keyboard", map[string]any{
					"keyboard_id": keyboardID,
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
