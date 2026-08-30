package keycapsets_test

import (
	"bytes"
	"net/http"

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
			var token string
			token, ownerID, err = api.NewAuthIdentity(ctx)
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

		Context("given the owner has a keycap set with a primary kit that has an image", func() {
			var (
				keycapSetID string
				kitID       string
			)

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "primary-kit-image-keycap-set-" + uuid.NewString()
				kitID = "kit-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())

				setResult, setErr := client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
					"keycap_set_id": keycapSetID,
					"kit_id":        kitID,
					"content_type":  approvedImageContentType,
				})
				Expect(setErr).NotTo(HaveOccurred())
				Expect(setResult.IsError).To(BeFalse())

				putResp, putErr := api.DoPresigned(ctx, http.MethodPut, decodeUploadURL(setResult), approvedImageContentType, bytes.NewReader([]byte("fake-image-bytes-for-list-testing")))
				Expect(putErr).NotTo(HaveOccurred())
				Expect(putResp.StatusCode).To(Equal(http.StatusOK))
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("reports primary_kit_id and primary_kit_has_image true, without a URL", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					found := false
					for _, ks := range out.KeycapSets {
						if ks.ID != keycapSetID {
							continue
						}
						found = true
						Expect(ks.PrimaryKitID).NotTo(BeNil())
						Expect(*ks.PrimaryKitID).To(Equal(kitID))
						Expect(ks.PrimaryKitHasImage).To(BeTrue())
					}
					Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
				})
			})
		})

		Context("given the owner has a keycap set whose primary kit no longer exists", func() {
			var keycapSetID string

			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "dangling-primary-kit-keycap-set-" + uuid.NewString()
				Expect(db.SeedKeycapSetWithDanglingPrimaryKit(ctx, ownerID, keycapSetID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			When("the list_keycap_sets tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_keycap_sets", map[string]any{})
				})

				It("reports a nil primary_kit_id and primary_kit_has_image false", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeListOutput(result)
					found := false
					for _, ks := range out.KeycapSets {
						if ks.ID != keycapSetID {
							continue
						}
						found = true
						Expect(ks.PrimaryKitID).To(BeNil())
						Expect(ks.PrimaryKitHasImage).To(BeFalse())
					}
					Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
				})
			})
		})

		Context("given another user owns keycap sets at every visibility tier", func() {
			var (
				otherID         string
				publicID        string
				authenticatedID string
				privateID       string
			)

			BeforeEach(func(ctx SpecContext) {
				_, otherID, err = api.NewAuthIdentity(ctx)
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
