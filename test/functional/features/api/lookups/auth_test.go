package lookups_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

var _ = Describe("Sending a malformed bearer token", func() {
	var (
		resp   *http.Response
		client *api.LookupsClient
	)

	BeforeEach(func() {
		resp = nil
		client = api.NewLookupsClient()
	})

	Context("given the caller sends a malformed bearer token", func() {
		When("calling a route that requires authentication", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.CreateCategory(ctx,
					"functional-test-category-"+uuid.NewString(),
					"not-a-valid-jwt",
					`{"values":["a"]}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
