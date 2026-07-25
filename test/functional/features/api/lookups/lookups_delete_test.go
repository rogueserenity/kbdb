package lookups_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Deleting a category", func() {
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
				Expect(db.SeedLookupCategory(ctx, category, []string{"a", "b"})).To(Succeed())
			})

			When("deleting the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.DeleteCategory(ctx, category, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("deletes the category", func(ctx SpecContext) {
					By("returning 204 No Content")
					Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

					By("actually removing the category, not a no-op")
					getResp, err := client.GetCategory(ctx, category)
					Expect(err).NotTo(HaveOccurred())
					Expect(getResp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})

		Context("given the category never existed", func() {
			When("deleting the category", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.DeleteCategory(ctx, category, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 204 (idempotent, not 404)", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
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

		When("deleting the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.DeleteCategory(ctx, category, token)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	Context("given the caller is anonymous", func() {
		When("deleting the category", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.DeleteCategory(ctx, category, "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
