package keycapsets_test

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Listing keycap sets over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given the owner has keycap sets at every visibility tier", func() {
			var publicID, authenticatedID, privateID string

			BeforeEach(func(ctx SpecContext) {
				publicID = "public-keycap-set-" + uuid.NewString()
				authenticatedID = "authenticated-keycap-set-" + uuid.NewString()
				privateID = "private-keycap-set-" + uuid.NewString()

				Expect(db.SeedKeycapSet(ctx, ownerID, publicID, "public")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, ownerID, authenticatedID, "authenticated")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, ownerID, privateID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, publicID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, ownerID, authenticatedID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, ownerID, privateID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("defaults to the caller's own collection and includes all three tiers", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := idsOf(decodeListOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID, privateID))
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("clamping rather than rejecting the call")
					Expect(result.IsError).To(BeFalse())

					ids := idsOf(decodeListOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID, privateID))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given another user owns keycap sets at every visibility tier", func() {
			var (
				otherID         string
				publicID        string
				authenticatedID string
				privateID       string
			)

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				publicID = "public-keycap-set-" + uuid.NewString()
				authenticatedID = "authenticated-keycap-set-" + uuid.NewString()
				privateID = "private-keycap-set-" + uuid.NewString()

				Expect(db.SeedKeycapSet(ctx, otherID, publicID, "public")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, otherID, authenticatedID, "authenticated")).To(Succeed())
				Expect(db.SeedKeycapSet(ctx, otherID, privateID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, publicID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, otherID, authenticatedID)).To(Succeed())
				Expect(db.DeleteKeycapSet(ctx, otherID, privateID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{"user_id": otherID})
				})

				It("returns the public and authenticated keycap sets, but not the private one", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					ids := idsOf(decodeListOutput(result))
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_keycap_sets tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
