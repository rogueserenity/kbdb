package ping_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

var _ = Describe("Ping", func() {
	var (
		client *api.Client
		token  string
		resp   *http.Response
	)

	BeforeEach(func() {
		client = api.NewClient()
		token = ""
	})

	Context("given no bearer token", func() {
		When("the request is made", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Do(ctx, http.MethodGet, "/v1/ping", token, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("rejects the request", func() {
				By("returning 401 Unauthorized")
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			var err error
			token, err = api.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("the request is made", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Do(ctx, http.MethodGet, "/v1/ping", token, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})
})
