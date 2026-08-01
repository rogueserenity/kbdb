package keycapsets_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a keycap set", func() {
	var (
		resp       *http.Response
		client     *api.KeycapSetsClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeycapSetsClient()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	seedKeycapSet := func(ctx SpecContext, visibility string) string {
		id := visibility + "-keycap-set-" + uuid.NewString()
		Expect(db.SeedKeycapSet(ctx, ownerID, id, visibility)).To(Succeed())
		return id
	}

	Context("given a public keycap set", func() {
		var keycapSetID string

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = seedKeycapSet(ctx, "public")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keycap set", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keycapSetID))
				})
			})
		})
	})

	Context("given an authenticated-only keycap set", func() {
		var keycapSetID string

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = seedKeycapSet(ctx, "authenticated")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keycap set", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keycapSetID))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given a private keycap set", func() {
		var keycapSetID string

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = seedKeycapSet(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keycap set", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the keycap set's id")
					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keycapSetID))
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

			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the keycap set", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keycapSetID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the keycap set does not exist", func() {
		When("getting the keycap set", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, "no-such-keycap-set-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
