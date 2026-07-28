package lookups_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Creating a category", func() {
	var (
		resp     *http.Response
		client   *api.LookupsClient
		category string
	)

	BeforeEach(func() {
		resp = nil
		client = api.NewLookupsClient()

		category = "functional-test-category-" + uuid.NewString()
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteLookupCategory(ctx, category)).To(Succeed())
	})

	Context("given the caller is an admin", func() {
		var token string

		BeforeEach(func(ctx SpecContext) {
			var err error
			token, err = api.AdminAuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("given the category does not exist", func() {
			When("creating the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.CreateCategory(ctx, category, token, `{"values":["a","b"]}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates the category", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning the created category's name and values")
					var got struct {
						Category string   `json:"category"`
						Values   []string `json:"values"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Category).To(Equal(category))
					Expect(got.Values).To(Equal([]string{"a", "b"}))
				})
			})
		})

		Context("given the category already exists", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
			})

			When("creating the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.CreateCategory(ctx, category, token, `{"values":["a","b"]}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 409 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusConflict))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given values is empty", func() {
			When("creating the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.CreateCategory(ctx, category, token, `{"values":[]}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the caller is not an admin", func() {
		var token string

		BeforeEach(func(ctx SpecContext) {
			var err error
			token, err = api.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.CreateCategory(ctx, category, token, `{"values":["a","b"]}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	Context("given the caller is anonymous", func() {
		When("creating the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.CreateCategory(ctx, category, "", `{"values":["a","b"]}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
