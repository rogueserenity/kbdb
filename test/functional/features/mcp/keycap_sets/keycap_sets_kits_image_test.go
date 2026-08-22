package keycapsets_test

import (
	"bytes"
	"io"
	"net/http"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Managing a keycap kit's image over MCP", func() {
	var (
		client      *api.MCPClient
		result      *sdkmcp.CallToolResult
		err         error
		ownerID     string
		keycapSetID string
		kitID       string
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

		Context("given the caller owns a keycap set with an existing kit", func() {
			BeforeEach(func(ctx SpecContext) {
				keycapSetID = "kit-image-set-" + uuid.NewString()
				Expect(db.SeedKeycapSet(ctx, ownerID, keycapSetID, "private")).To(Succeed())

				createResult, createErr := client.CallTool(ctx, "create_keycap_kit", map[string]any{
					"keycap_set_id": keycapSetID,
					"name":          "Base",
				})
				Expect(createErr).NotTo(HaveOccurred())
				Expect(createResult.IsError).To(BeFalse())
				kitID = decodeKeycapKitOutput(createResult).KeycapKit.KitID
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
			})

			Context("given an approved content_type", func() {
				When("the set_keycap_kit_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
							"content_type":  approvedImageContentType,
						})
					})

					It("mints an upload_url that a real PUT-then-GET round-trip actually works against", func(ctx SpecContext) {
						By("returning an upload_url")
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())
						uploadURL := decodeUploadURL(result)
						Expect(uploadURL).NotTo(BeEmpty())

						By("uploading arbitrary bytes to the presigned PUT URL")
						imageBytes := []byte("fake-image-bytes-for-testing")
						putResp, putErr := api.DoPresigned(ctx, http.MethodPut, uploadURL, approvedImageContentType, bytes.NewReader(imageBytes))
						Expect(putErr).NotTo(HaveOccurred())
						Expect(putResp.StatusCode).To(Equal(http.StatusOK))

						By("has_image reflecting it on a follow-up get_keycap_set")
						check, checkErr := client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
						Expect(checkErr).NotTo(HaveOccurred())
						kits := decodeKeycapSetOutput(check).KeycapSet.Kits
						Expect(kits).To(HaveLen(1))
						Expect(kits[0].HasImage).To(BeTrue())

						By("get_keycap_kit_image_url minting a presigned GET URL that returns the exact bytes uploaded")
						urlResult, urlErr := client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
						})
						Expect(urlErr).NotTo(HaveOccurred())
						Expect(urlResult.IsError).To(BeFalse())
						imageURL := decodeImageURL(urlResult)
						Expect(imageURL).NotTo(BeEmpty())

						getImageResp, getErr := api.DoPresigned(ctx, http.MethodGet, imageURL, "", nil)
						Expect(getErr).NotTo(HaveOccurred())
						Expect(getImageResp.StatusCode).To(Equal(http.StatusOK))

						gotBytes, readErr := io.ReadAll(getImageResp.Body)
						Expect(readErr).NotTo(HaveOccurred())
						Expect(gotBytes).To(Equal(imageBytes))
					})
				})
			})

			Context("given an unapproved content_type", func() {
				When("the set_keycap_kit_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
							"content_type":  "application/x-not-an-image",
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})
			})

			Context("given the kit_id does not exist within the set", func() {
				When("the set_keycap_kit_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        "no-such-kit-" + uuid.NewString(),
							"content_type":  approvedImageContentType,
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})
			})

			Context("given the kit has no image set", func() {
				When("the get_keycap_kit_image_url tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})

				When("the delete_keycap_kit_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "delete_keycap_kit_image", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
						})
					})

					It("succeeds, since deleting an absent image is idempotent", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())
					})
				})
			})

			Context("given the kit has an image set", func() {
				var imageGetURL string

				BeforeEach(func(ctx SpecContext) {
					setResult, setErr := client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
						"keycap_set_id": keycapSetID,
						"kit_id":        kitID,
						"content_type":  approvedImageContentType,
					})
					Expect(setErr).NotTo(HaveOccurred())
					Expect(setResult.IsError).To(BeFalse())
					uploadURL := decodeUploadURL(setResult)

					putResp, putErr := api.DoPresigned(ctx, http.MethodPut, uploadURL, approvedImageContentType, bytes.NewReader([]byte("fake-image-bytes")))
					Expect(putErr).NotTo(HaveOccurred())
					Expect(putResp.StatusCode).To(Equal(http.StatusOK))

					urlResult, urlErr := client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
						"keycap_set_id": keycapSetID,
						"kit_id":        kitID,
					})
					Expect(urlErr).NotTo(HaveOccurred())
					Expect(urlResult.IsError).To(BeFalse())
					imageGetURL = decodeImageURL(urlResult)
				})

				When("the delete_keycap_kit_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "delete_keycap_kit_image", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
						})
					})

					It("removes the image, and it's actually gone from S3", func(ctx SpecContext) {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())

						By("get_keycap_kit_image_url now tool-erroring")
						check, checkErr := client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
							"keycap_set_id": keycapSetID,
							"kit_id":        kitID,
						})
						Expect(checkErr).NotTo(HaveOccurred())
						Expect(check.IsError).To(BeTrue())

						By("the previously-issued presigned GET URL no longer serving the object from S3")
						getImageResp, getErr := api.DoPresigned(ctx, http.MethodGet, imageGetURL, "", nil)
						Expect(getErr).NotTo(HaveOccurred())
						// Real S3 returns 403 (not 404) for GetObject against a
						// missing key when the caller lacks s3:ListBucket;
						// LocalStack returns 404.
						Expect(getImageResp.StatusCode).To(BeElementOf(http.StatusNotFound, http.StatusForbidden))
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

				keycapSetID = "kit-image-set-" + uuid.NewString()
				Expect(db.SeedKeycapSet(ctx, otherID, keycapSetID, "public")).To(Succeed())

				otherClient := api.NewMCPClient(support.BaseURL()+"/mcp", otherToken)
				createResult, createErr := otherClient.CallTool(ctx, "create_keycap_kit", map[string]any{
					"keycap_set_id": keycapSetID,
					"name":          "Base",
				})
				Expect(createErr).NotTo(HaveOccurred())
				Expect(createResult.IsError).To(BeFalse())
				kitID = decodeKeycapKitOutput(createResult).KeycapKit.KitID
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the set_keycap_kit_image tool is called with that keycap_set_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
						"keycap_set_id": keycapSetID,
						"kit_id":        kitID,
						"content_type":  approvedImageContentType,
					})
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given the parent keycap set does not exist", func() {
			When("the set_keycap_kit_image tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
						"keycap_set_id": "no-such-keycap-set-" + uuid.NewString(),
						"kit_id":        "some-kit-id",
						"content_type":  approvedImageContentType,
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

		When("the set_keycap_kit_image tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
					"keycap_set_id": "irrelevant-" + uuid.NewString(),
					"kit_id":        "irrelevant",
					"content_type":  approvedImageContentType,
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		When("the get_keycap_kit_image_url tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
					"keycap_set_id": "irrelevant-" + uuid.NewString(),
					"kit_id":        "irrelevant",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})

		When("the delete_keycap_kit_image tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_keycap_kit_image", map[string]any{
					"keycap_set_id": "irrelevant-" + uuid.NewString(),
					"kit_id":        "irrelevant",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
