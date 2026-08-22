package keyboards_test

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

var _ = Describe("Adding an image to a keyboard over MCP", func() {
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

			Context("given an approved content_type", func() {
				When("the add_keyboard_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "add_keyboard_image", map[string]any{
							"keyboard_id":  keyboardID,
							"content_type": approvedImageContentType,
						})
					})

					It("mints an image_id and upload_url that a real PUT round-trip actually works against", func(ctx SpecContext) {
						By("returning an image_id and upload_url")
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeFalse())
						out := decodeAddImageOutput(result)
						Expect(out.ImageID).NotTo(BeEmpty())
						Expect(out.UploadURL).NotTo(BeEmpty())

						By("uploading arbitrary bytes to the presigned PUT URL")
						imageBytes := []byte("fake-image-bytes-for-testing")
						putResp, putErr := api.DoPresigned(ctx, http.MethodPut, out.UploadURL, approvedImageContentType, bytes.NewReader(imageBytes))
						Expect(putErr).NotTo(HaveOccurred())
						Expect(putResp.StatusCode).To(Equal(http.StatusOK))

						By("has_images reflecting it on a follow-up get_keyboard")
						check, checkErr := client.CallTool(ctx, "get_keyboard", map[string]any{"keyboard_id": keyboardID})
						Expect(checkErr).NotTo(HaveOccurred())
						Expect(check.IsError).To(BeFalse())
						Expect(decodeGetOutput(check).Keyboard.HasImages).To(BeTrue())

						By("get_keyboard_image_url minting a presigned GET URL that returns the exact bytes uploaded")
						urlResult, urlErr := client.CallTool(ctx, "get_keyboard_image_url", map[string]any{
							"keyboard_id": keyboardID,
							"image_id":    out.ImageID,
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

			Context("given the keyboard has no image set", func() {
				When("the get_keyboard_image_url tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "get_keyboard_image_url", map[string]any{
							"keyboard_id": keyboardID,
							"image_id":    "no-such-image-" + uuid.NewString(),
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
				})
			})

			Context("given an unapproved content_type", func() {
				When("the add_keyboard_image tool is called", func() {
					BeforeEach(func(ctx SpecContext) {
						result, err = client.CallTool(ctx, "add_keyboard_image", map[string]any{
							"keyboard_id":  keyboardID,
							"content_type": "application/x-not-an-image",
						})
					})

					It("returns an MCP tool error result", func() {
						Expect(err).NotTo(HaveOccurred())
						Expect(result.IsError).To(BeTrue())
					})
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

			When("the add_keyboard_image tool is called with that keyboard_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "add_keyboard_image", map[string]any{
						"keyboard_id":  keyboardID,
						"content_type": approvedImageContentType,
					})
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given the keyboard does not exist", func() {
			When("the add_keyboard_image tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "add_keyboard_image", map[string]any{
						"keyboard_id":  "no-such-keyboard-" + uuid.NewString(),
						"content_type": approvedImageContentType,
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

		When("the add_keyboard_image tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "add_keyboard_image", map[string]any{
					"keyboard_id":  "irrelevant-" + uuid.NewString(),
					"content_type": approvedImageContentType,
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
