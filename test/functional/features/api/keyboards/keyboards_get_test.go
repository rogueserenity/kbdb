package keyboards_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a keyboard", func() {
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

	seedKeyboard := func(ctx SpecContext, visibility string) string {
		id := visibility + "-keyboard-" + uuid.NewString()
		Expect(db.SeedKeyboard(ctx, ownerID, id, visibility)).To(Succeed())
		return id
	}

	Context("given a public keyboard", func() {
		var keyboardID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID = seedKeyboard(ctx, "public")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keyboard without purchase.price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID       string `json:"id"`
						Purchase struct {
							Vendor *string  `json:"vendor"`
							Price  *float64 `json:"price"`
						} `json:"purchase"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keyboardID))

					By("still including non-price purchase fields")
					Expect(got.Purchase.Vendor).NotTo(BeNil())
					Expect(*got.Purchase.Vendor).To(Equal("Amazon"))

					By("omitting price")
					Expect(got.Purchase.Price).To(BeNil())
				})
			})
		})

		Context("given the caller is the owner", func() {
			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keyboard with purchase.price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						Purchase struct {
							Price *float64 `json:"price"`
						} `json:"purchase"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())

					Expect(got.Purchase.Price).NotTo(BeNil())
					Expect(*got.Purchase.Price).To(Equal(329.99))
				})
			})
		})
	})

	Context("given an authenticated-only keyboard", func() {
		var keyboardID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID = seedKeyboard(ctx, "authenticated")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keyboard", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keyboardID))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given a private keyboard", func() {
		var keyboardID string

		BeforeEach(func(ctx SpecContext) {
			keyboardID = seedKeyboard(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the keyboard", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the keyboard's id")
					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(keyboardID))
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

			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, keyboardID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the keyboard does not exist", func() {
		When("getting the keyboard", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, "no-such-keyboard-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
