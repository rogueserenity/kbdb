package keycapsets_test

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Listing keycap sets", func() {
	var (
		resp       *http.Response
		client     *api.KeycapSetsClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeycapSetsClient()

		// Derived from a freshly minted token, not a fixed fixture subject:
		// in CI, AuthToken mints a real subject from the WorkOS emulator
		// rather than a fixed fixture subject string, so the owner used
		// to seed fixture data below must match whatever subject this
		// environment's token actually carries.
		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	itemIDs := func(r *http.Response) []string {
		var page struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
		ids := make([]string, len(page.Items))
		for i, item := range page.Items {
			ids[i] = item.ID
		}
		return ids
	}

	Context("given the owner has keycap sets at every visibility tier", func() {
		var publicID, authenticatedID, privateID string

		BeforeEach(func(ctx SpecContext) {
			// Fresh IDs per spec run, not fixed literals: AuthToken is a
			// shared, real emulator identity in CI (provisioning one per
			// spec isn't practical there), so specs sharing an identity
			// must namespace their own data to stay collision-proof under
			// concurrent/out-of-order runs.
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

		Context("given the caller is the owner", func() {
			When("listing keycap sets", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns keycap sets at every visibility tier", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("including all three seeded keycap sets")
					Expect(itemIDs(resp)).To(ContainElements(publicID, authenticatedID, privateID))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("listing keycap sets", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the public keycap set", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(resp)
					Expect(ids).To(ContainElement(publicID))
					Expect(ids).NotTo(ContainElement(authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, _, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("listing keycap sets", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the public and authenticated keycap sets, but not the private one", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(resp)
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})
	})

	Context("given the owner has a keycap set with a primary kit that has an image", func() {
		var (
			keycapSetID string
			kitID       string
			imageBytes  []byte
		)

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = "primary-kit-image-keycap-set-" + uuid.NewString()
			kitID = "kit-" + uuid.NewString()
			Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())

			setImageResp, err := client.SetKitImage(ctx, ownerID, keycapSetID, kitID, ownerToken, `{"content_type":"image/png"}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(setImageResp.StatusCode).To(Equal(http.StatusCreated))

			var created struct {
				UploadURL string `json:"upload_url"`
			}
			Expect(json.NewDecoder(setImageResp.Body).Decode(&created)).To(Succeed())

			imageBytes = []byte("fake-image-bytes-for-list-testing")
			putResp, err := api.DoPresigned(ctx, http.MethodPut, created.UploadURL, "image/png", bytes.NewReader(imageBytes))
			Expect(err).NotTo(HaveOccurred())
			Expect(putResp.StatusCode).To(Equal(http.StatusOK))
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		When("listing keycap sets", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, -1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("includes a working presigned primary_kit_image URL for that set", func(ctx SpecContext) {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var page struct {
					Items []struct {
						ID              string `json:"id"`
						PrimaryKitImage *struct {
							URL string `json:"url"`
						} `json:"primary_kit_image"`
					} `json:"items"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())

				var found bool
				for _, item := range page.Items {
					if item.ID != keycapSetID {
						continue
					}
					found = true
					Expect(item.PrimaryKitImage).NotTo(BeNil())
					Expect(item.PrimaryKitImage.URL).NotTo(BeEmpty())

					getImageResp, err := api.DoPresigned(ctx, http.MethodGet, item.PrimaryKitImage.URL, "", nil)
					Expect(err).NotTo(HaveOccurred())
					Expect(getImageResp.StatusCode).To(Equal(http.StatusOK))
				}
				Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
			})
		})
	})

	Context("given the owner has a keycap set with a primary kit that has no image", func() {
		var (
			keycapSetID string
			kitID       string
		)

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = "primary-kit-no-image-keycap-set-" + uuid.NewString()
			kitID = "kit-" + uuid.NewString()
			Expect(db.SeedKeycapSetWithPrimaryKit(ctx, ownerID, keycapSetID, kitID, "public")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		When("listing keycap sets", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, -1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("reports a null primary_kit_image for that set", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var page struct {
					Items []struct {
						ID              string `json:"id"`
						PrimaryKitImage *struct {
							URL string `json:"url"`
						} `json:"primary_kit_image"`
					} `json:"items"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())

				var found bool
				for _, item := range page.Items {
					if item.ID != keycapSetID {
						continue
					}
					found = true
					Expect(item.PrimaryKitImage).To(BeNil())
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

		When("listing keycap sets", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, -1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("reports a null primary_kit_image for that set", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var page struct {
					Items []struct {
						ID              string `json:"id"`
						PrimaryKitImage *struct {
							URL string `json:"url"`
						} `json:"primary_kit_image"`
					} `json:"items"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&page)).To(Succeed())

				var found bool
				for _, item := range page.Items {
					if item.ID != keycapSetID {
						continue
					}
					found = true
					Expect(item.PrimaryKitImage).To(BeNil())
				}
				Expect(found).To(BeTrue(), "expected to find seeded keycap set %q in the list", keycapSetID)
			})
		})
	})

	DescribeTable("given an invalid limit",
		func(ctx SpecContext, limit int) {
			var err error
			resp, err = client.List(ctx, ownerID, ownerToken, limit)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
		},
		Entry("below the minimum", 0),
		Entry("above the maximum", 101),
	)

	Context("given a non-numeric limit", func() {
		When("listing keycap sets", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ListWithRawLimit(ctx, ownerID, ownerToken, "abc")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
