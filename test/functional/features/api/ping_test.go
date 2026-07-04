package api_test

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var _ = Describe("GET /v1/ping", func() {
	It("returns 401 without a bearer token", func() {
		resp, err := http.Get(support.BaseURL() + "/v1/ping")
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()

		Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
	})

	It("returns 200 with a valid bearer token", func() {
		token, err := support.AuthToken()
		Expect(err).NotTo(HaveOccurred())

		req, err := http.NewRequest(http.MethodGet, support.BaseURL()+"/v1/ping", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = resp.Body.Close() }()

		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})
})
