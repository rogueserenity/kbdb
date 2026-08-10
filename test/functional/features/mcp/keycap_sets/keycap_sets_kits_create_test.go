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

var _ = Describe("Creating a keycap kit over MCP", func() {
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
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given the caller owns a keycap set", func() {
			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "create-kit-set-" + uuid.NewString()
				Expect(db.SeedKeycapSet(ctx, ownerID, keycapSetID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the create_keycap_kit tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
						"keycap_set_id": keycapSetID,
						"name":          "Base",
					})
				})

				It("creates the kit, embedded in the parent set", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeKeycapKitOutput(result)
					Expect(out.KeycapKit.KitID).NotTo(BeEmpty())
					Expect(out.KeycapKit.Name).To(Equal("Base"))
					Expect(out.KeycapKit.HasImage).To(BeFalse())

					By("showing the kit embedded in a follow-up get_keycap_set")
					check, checkErr := client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
					Expect(checkErr).NotTo(HaveOccurred())

					kits := decodeKeycapSetOutput(check).KeycapSet.Kits
					Expect(kits).To(HaveLen(1))
					Expect(kits[0].KitID).To(Equal(out.KeycapKit.KitID))
					Expect(kits[0].Name).To(Equal("Base"))
				})
			})

			Context("given every open-vocabulary purchase field has an approved lookup value", func() {
				When("the create_keycap_kit tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
							"keycap_set_id": keycapSetID,
							"name":          "Base",
							"purchase": map[string]any{
								"vendor":       approvedVendor,
								"order_status": approvedOrderStatus,
							},
						})
					})

					It("creates the kit", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())
					})
				})
			})

			Context("given a purchase field has an unapproved value", func() {
				When("the create_keycap_kit tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
							"keycap_set_id": keycapSetID,
							"name":          "Base",
							"purchase": map[string]any{
								"vendor": "NotApproved",
							},
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})
			})

			Context("given a required field is present but blank", func() {
				When("the create_keycap_kit tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
							"keycap_set_id": keycapSetID,
							"name":          "",
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})
			})

			Context("given the set already has one kit", func() {
				BeforeEach(func(ctx SpecContext) {
					createResult, createErr := client.CallTool(ctx, "create_keycap_kit", map[string]any{
						"keycap_set_id": keycapSetID,
						"name":          "Base",
					})
					Expect(createErr).NotTo(HaveOccurred())
					Expect(createResult.IsError).To(BeFalse())
				})

				When("the create_keycap_kit tool is called for a second kit", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
							"keycap_set_id": keycapSetID,
							"name":          "Extension",
						})
					})

					It("accumulates kits rather than replacing the existing one", func(ctx SpecContext) {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())

						check, checkErr := client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
						Expect(checkErr).NotTo(HaveOccurred())

						kits := decodeKeycapSetOutput(check).KeycapSet.Kits
						Expect(kits).To(HaveLen(2))

						names := make([]string, len(kits))
						for i, k := range kits {
							names[i] = k.Name
						}
						Expect(names).To(ConsistOf("Base", "Extension"))
					})
				})
			})
		})

		Context("given another user owns the keycap set", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())

				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				keycapSetID = "create-kit-set-" + uuid.NewString()
				Expect(db.SeedKeycapSet(ctx, otherID, keycapSetID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the create_keycap_kit tool is called with that keycap_set_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
						"keycap_set_id": keycapSetID,
						"name":          "Hijacked",
					})
				})

				It("returns an MCP tool error result and adds no kit", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())

					otherToken, tokenErr := api.SecondUserAuthToken(ctx)
					Expect(tokenErr).NotTo(HaveOccurred())
					otherClient := api.NewMCPClient(support.BaseURL()+"/mcp", otherToken)

					check, checkErr := otherClient.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
					Expect(checkErr).NotTo(HaveOccurred())
					Expect(decodeKeycapSetOutput(check).KeycapSet.Kits).To(BeEmpty())
				})
			})
		})

		Context("given the parent keycap set does not exist", func() {
			When("the create_keycap_kit tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
						"keycap_set_id": "no-such-keycap-set-" + uuid.NewString(),
						"name":          "Base",
					})
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

		When("the create_keycap_kit tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_keycap_kit", map[string]any{
					"keycap_set_id": "irrelevant-" + uuid.NewString(),
					"name":          "Base",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
