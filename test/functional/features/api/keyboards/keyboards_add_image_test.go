package keyboards_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Adding an image to a keyboard", func() {
	var (
		resp       *http.Response
		client     *api.KeyboardsClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeyboardsClient()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a private keyboard owned by the caller", func() {
		var keyboardID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID = "add-image-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			Context("given an approved content_type", func() {
				When("adding an image to the keyboard", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.AddImage(ctx, ownerID, keyboardID, ownerToken, `{"content_type":"`+approvedImageContentType+`"}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns an image_id and an upload URL that a real PUT-then-GET round-trip actually works against", func(ctx SpecContext) {
						By("returning 201 with image_id and upload_url")
						Expect(resp.StatusCode).To(Equal(http.StatusCreated))

						var created struct {
							ImageID   string `json:"image_id"`
							UploadURL string `json:"upload_url"`
						}
						Expect(json.NewDecoder(resp.Body).Decode(&created)).To(Succeed())
						Expect(created.ImageID).NotTo(BeEmpty())
						Expect(created.UploadURL).NotTo(BeEmpty())

						By("uploading arbitrary bytes to the presigned PUT URL")
						imageBytes := []byte("fake-image-bytes-for-testing")
						putResp, err := api.DoPresigned(ctx, http.MethodPut, created.UploadURL, approvedImageContentType, bytes.NewReader(imageBytes))
						Expect(err).NotTo(HaveOccurred())
						Expect(putResp.StatusCode).To(Equal(http.StatusOK))

						By("showing the image, as a presigned GET URL, embedded in a follow-up GetKeyboard")
						getResp, err := client.Get(ctx, ownerID, keyboardID, ownerToken)
						Expect(err).NotTo(HaveOccurred())

						var keyboard struct {
							Images []struct {
								ImageID string `json:"image_id"`
								URL     string `json:"url"`
							} `json:"images"`
						}
						Expect(json.NewDecoder(getResp.Body).Decode(&keyboard)).To(Succeed())
						Expect(keyboard.Images).To(HaveLen(1))
						Expect(keyboard.Images[0].ImageID).To(Equal(created.ImageID))
						Expect(keyboard.Images[0].URL).NotTo(BeEmpty())

						By("fetching the presigned GET URL and getting back the exact bytes that were uploaded")
						getImageResp, err := api.DoPresigned(ctx, http.MethodGet, keyboard.Images[0].URL, "", nil)
						Expect(err).NotTo(HaveOccurred())
						Expect(getImageResp.StatusCode).To(Equal(http.StatusOK))

						gotBytes, err := io.ReadAll(getImageResp.Body)
						Expect(err).NotTo(HaveOccurred())
						Expect(gotBytes).To(Equal(imageBytes))
					})
				})
			})

			Context("given an unapproved content_type", func() {
				When("adding an image to the keyboard", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.AddImage(ctx, ownerID, keyboardID, ownerToken, `{"content_type":"application/x-not-an-image"}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns 400 with a problem+json body", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
					})
				})
			})

			Context("given required fields are missing", func() {
				When("adding an image to the keyboard", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.AddImage(ctx, ownerID, keyboardID, ownerToken, `{}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns 400 with a problem+json body", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
					})
				})
			})
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("adding an image to the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.AddImage(ctx, ownerID, keyboardID, token, `{"content_type":"`+approvedImageContentType+`"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the keyboard exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("adding an image to the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.AddImage(ctx, ownerID, keyboardID, "", `{"content_type":"`+approvedImageContentType+`"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})
	})

	Context("given the keyboard does not exist", func() {
		When("adding an image to the keyboard", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.AddImage(ctx, ownerID, "no-such-keyboard-"+uuid.NewString(), ownerToken, `{"content_type":"`+approvedImageContentType+`"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
