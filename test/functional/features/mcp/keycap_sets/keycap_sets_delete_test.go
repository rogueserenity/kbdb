package keycapsets_test

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Deleting a keycap set over MCP", func() {
	var (
		client      *api.MCPClient
		result      *sdkmcp.CallToolResult
		err         error
		ownerID     string
		keycapSetID string
		deleted     bool
	)

	BeforeEach(func() {
		result = nil
		err = nil
		deleted = false
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
				if !deleted {
					Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
				}
			})

			When("the delete_keycap_set tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
					if err == nil && !result.IsError {
						deleted = true
					}
				})

				It("removes the keycap set", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					By("a subsequent get_keycap_set no longer finding it")
					check, checkErr := client.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
					Expect(checkErr).NotTo(HaveOccurred())
					Expect(check.IsError).To(BeTrue())
				})
			})

			Context("given the set has a kit with an image", func() {
				var imageGetURL string

				BeforeEach(func(ctx SpecContext) {
					kitResult, kitErr := client.CallTool(ctx, "create_keycap_kit", map[string]any{
						"keycap_set_id": keycapSetID,
						"name":          "Base",
					})
					Expect(kitErr).NotTo(HaveOccurred())
					Expect(kitResult.IsError).To(BeFalse())
					kitID := decodeKeycapKitOutput(kitResult).KeycapKit.KitID

					setImgResult, setImgErr := client.CallTool(ctx, "set_keycap_kit_image", map[string]any{
						"keycap_set_id": keycapSetID,
						"kit_id":        kitID,
						"content_type":  approvedImageContentType,
					})
					Expect(setImgErr).NotTo(HaveOccurred())
					Expect(setImgResult.IsError).To(BeFalse())

					var upload struct {
						UploadURL string `json:"upload_url"`
					}
					raw, marshalErr := json.Marshal(setImgResult.StructuredContent)
					Expect(marshalErr).NotTo(HaveOccurred())
					Expect(json.Unmarshal(raw, &upload)).To(Succeed())

					putResp, putErr := api.DoPresigned(ctx, http.MethodPut, upload.UploadURL, approvedImageContentType, bytes.NewReader([]byte("fake-image-bytes")))
					Expect(putErr).NotTo(HaveOccurred())
					Expect(putResp.StatusCode).To(Equal(http.StatusOK))

					imgURLResult, imgURLErr := client.CallTool(ctx, "get_keycap_kit_image_url", map[string]any{
						"keycap_set_id": keycapSetID,
						"kit_id":        kitID,
					})
					Expect(imgURLErr).NotTo(HaveOccurred())
					Expect(imgURLResult.IsError).To(BeFalse())

					var got struct {
						URL string `json:"url"`
					}
					raw2, marshalErr2 := json.Marshal(imgURLResult.StructuredContent)
					Expect(marshalErr2).NotTo(HaveOccurred())
					Expect(json.Unmarshal(raw2, &got)).To(Succeed())
					imageGetURL = got.URL
				})

				When("the delete_keycap_set tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "delete_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
						if err == nil && !result.IsError {
							deleted = true
						}
					})

					It("removes the set and the kit's image is actually gone from S3", func(ctx SpecContext) {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())

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

		Context("given the keycap set never existed", func() {
			When("the delete_keycap_set tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
				})

				It("succeeds, since deleting is idempotent", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
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

				Expect(db.SeedKeycapSet(ctx, otherID, keycapSetID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteKeycapSet(ctx, otherID, keycapSetID)).To(Succeed())
			})

			When("the delete_keycap_set tool is called with that id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "delete_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
				})

				It("leaves the other user's keycap set in place", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					otherToken, tokenErr := api.SecondUserAuthToken(ctx)
					Expect(tokenErr).NotTo(HaveOccurred())
					otherClient := api.NewMCPClient(support.BaseURL()+"/mcp", otherToken)

					check, checkErr := otherClient.CallTool(ctx, "get_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
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

		When("the delete_keycap_set tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "delete_keycap_set", map[string]any{"keycap_set_id": keycapSetID})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
