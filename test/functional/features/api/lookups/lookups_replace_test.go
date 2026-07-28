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

var _ = Describe("Replacing a category", func() {
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

		Context("given the category exists", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
			})

			When("replacing the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.ReplaceCategory(ctx, category, token, `{"values":["c","d"]}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("replaces the category's values", func(ctx SpecContext) {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the new values in the response body")
					var got struct {
						Category string   `json:"category"`
						Values   []string `json:"values"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Category).To(Equal(category))
					Expect(got.Values).To(Equal([]string{"c", "d"}))

					By("actually persisting the new values, not a no-op")
					getResp, err := client.GetCategory(ctx, category)
					Expect(err).NotTo(HaveOccurred())
					Expect(getResp.StatusCode).To(Equal(http.StatusOK))

					var reGot struct {
						Values []string `json:"values"`
					}
					Expect(json.NewDecoder(getResp.Body).Decode(&reGot)).To(Succeed())
					Expect(reGot.Values).To(Equal([]string{"c", "d"}))
				})
			})
		})

		Context("given the category does not exist", func() {
			When("replacing the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.ReplaceCategory(ctx, category, token, `{"values":["c","d"]}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the category exists and values is empty", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
			})

			When("replacing the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.ReplaceCategory(ctx, category, token, `{"values":[]}`)
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
			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())

			var err error
			token, err = api.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("replacing the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ReplaceCategory(ctx, category, token, `{"values":["c","d"]}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	Context("given the caller is anonymous", func() {
		When("replacing the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ReplaceCategory(ctx, category, "", `{"values":["c","d"]}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
