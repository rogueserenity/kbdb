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

var _ = Describe("Getting a category", func() {
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

	Context("given the category exists", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("getting the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.GetCategory(ctx, category)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds and returns the category's values", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("returning the category's name and values")
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

	Context("given the category does not exist", func() {
		When("getting the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.GetCategory(ctx, "functional-test-category-missing-"+uuid.NewString())
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
