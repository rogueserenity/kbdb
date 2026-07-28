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

var _ = Describe("Listing categories", func() {
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

	Context("given the lookup table has a category", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedLookupCategory(ctx, category, []any{"a", "b"})).To(Succeed())
		})

		When("listing categories", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ListCategories(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds and includes the seeded category", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("including the seeded category name in the response body")
				var categories []string
				Expect(json.NewDecoder(resp.Body).Decode(&categories)).To(Succeed())
				Expect(categories).To(ContainElement(category))
			})
		})
	})
})
