package keycapsets_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a keycap set over MCP", func() {
	var (
		client      *api.MCPClient
		result      *sdkmcp.CallToolResult
		err         error
		ownerID     string
		keycapSetID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		keycapSetID = "functional-test-keycap-set-" + uuid.NewString()
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given the caller owns the keycap set", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedKeycapSet(ctx, ownerID, keycapSetID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the get_keycap_set tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
				})

				It("defaults to the caller's own collection and returns the keycap set", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeKeycapSetOutput(result)
					Expect(out.KeycapSet.ID).To(Equal(keycapSetID))
					Expect(out.KeycapSet.Brand).To(Equal("GMK"))
					Expect(out.KeycapSet.Visibility).To(Equal("private"))
				})
			})
		})

		Context("given the caller owns the keycap set and it has a kit", func() {
			var kitID string

			BeforeEach(func(ctx SpecContext) {
				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithKit(ctx, ownerID, keycapSetID, kitID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the get_keycap_set tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
				})

				It("includes the kit's purchase.price for the owner", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeKeycapSetOutput(result)
					Expect(out.KeycapSet.Kits).To(HaveLen(1))
					Expect(out.KeycapSet.Kits[0].Purchase).NotTo(BeNil())
					Expect(out.KeycapSet.Kits[0].Purchase.Price).NotTo(BeNil())
					Expect(*out.KeycapSet.Kits[0].Purchase.Price).To(Equal(85.0))
				})
			})
		})

		Context("given the keycap set never existed", func() {
			When("the get_keycap_set tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
				})

				It("returns an MCP tool error result, not a transport failure", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a private keycap set", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				Expect(db.SeedKeycapSet(ctx, otherID, keycapSetID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the get_keycap_set tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{
						"keycap_set_id": keycapSetID,
						"user_id":       otherID,
					})
				})

				It("is indistinguishable from the keycap set not existing", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given another user owns a public keycap set", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				Expect(db.SeedKeycapSet(ctx, otherID, keycapSetID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the get_keycap_set tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{
						"keycap_set_id": keycapSetID,
						"user_id":       otherID,
					})
				})

				It("returns the keycap set", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeKeycapSetOutput(result).KeycapSet.ID).To(Equal(keycapSetID))
				})
			})
		})

		Context("given another user owns a public keycap set with a kit", func() {
			var (
				otherID string
				kitID   string
			)

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithKit(ctx, otherID, keycapSetID, kitID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the get_keycap_set tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{
						"keycap_set_id": keycapSetID,
						"user_id":       otherID,
					})
				})

				It("returns the kit with purchase.price omitted", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeKeycapSetOutput(result)
					Expect(out.KeycapSet.Kits).To(HaveLen(1))

					By("still including non-price purchase fields")
					Expect(out.KeycapSet.Kits[0].Purchase).NotTo(BeNil())
					Expect(*out.KeycapSet.Kits[0].Purchase.Vendor).To(Equal("MechMarket"))

					By("omitting price")
					Expect(out.KeycapSet.Kits[0].Purchase.Price).To(BeNil())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the get_keycap_set tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
