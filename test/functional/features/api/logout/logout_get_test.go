package logout_test

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

// Not part of api/openapi.yaml, so tested here directly rather than via a
// generated entity client - see internal/consent.
var _ = Describe("Fetching the Stytch logout page", func() {
	var (
		client *api.Client
		resp   *http.Response
		err    error
	)

	BeforeEach(func() {
		client = api.NewClient()
		resp = nil
		err = nil
	})

	Context("given no Authorization header", func() {
		Context("given an allowed return_to origin", func() {
			When("the page is fetched", func() {
				BeforeEach(func(ctx SpecContext) {
					// Matches scripts/func-setup.sh's LogoutReturnOrigins
					// deploy parameter for this stack.
					resp, err = client.Do(ctx, http.MethodGet,
						"/logout?return_to=http://localhost:5173/", "", nil)
				})

				It("returns the page, not a 401", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(resp.Header.Get("Content-Type")).To(Equal("text/html; charset=utf-8"))
				})

				It("renders this stack's Stytch public token and return_to into the page", func() {
					body, readErr := io.ReadAll(resp.Body)
					Expect(readErr).NotTo(HaveOccurred())
					Expect(string(body)).To(ContainSubstring(`"public-token-test-local-kbdb"`))
					Expect(string(body)).To(ContainSubstring(`"http://localhost:5173/"`))
				})
			})
		})

		Context("given a disallowed return_to origin", func() {
			When("the page is fetched", func() {
				BeforeEach(func(ctx SpecContext) {
					resp, err = client.Do(ctx, http.MethodGet,
						"/logout?return_to=https://evil.example.com/", "", nil)
				})

				It("returns 400 Bad Request", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})

		Context("given no return_to param", func() {
			When("the page is fetched", func() {
				BeforeEach(func(ctx SpecContext) {
					resp, err = client.Do(ctx, http.MethodGet, "/logout", "", nil)
				})

				It("returns 400 Bad Request", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				})
			})
		})
	})

	Context("given a POST request", func() {
		When("the page is requested", func() {
			BeforeEach(func(ctx SpecContext) {
				resp, err = client.Do(ctx, http.MethodPost,
					"/logout?return_to=http://localhost:5173/", "", nil)
			})

			It("returns 405 Method Not Allowed", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))
			})
		})
	})
})
