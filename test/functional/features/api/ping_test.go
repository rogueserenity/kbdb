package api_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var _ = Describe("Ping", func() {
	var (
		authHeader string
		resp       *http.Response
	)

	BeforeEach(func() {
		authHeader = ""
		resp = nil
	})

	AfterEach(func() {
		if resp != nil {
			_ = resp.Body.Close()
		}
	})

	Context("given no bearer token", func() {
		When("the request is made", func() {
			BeforeEach(func(ctx SpecContext) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/ping", nil)
				Expect(err).NotTo(HaveOccurred())

				resp, err = http.DefaultClient.Do(req)
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
			token, err := support.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			authHeader = "Bearer " + token
		})

		When("the request is made", func() {
			BeforeEach(func(ctx SpecContext) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/ping", nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", authHeader)

				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})
})
