package auth_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Sending a malformed bearer token", func() {
	var (
		resp    *http.Response
		client  *api.SwitchesClient
		ownerID string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()

		ownerToken, err := api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given the caller sends a malformed bearer token", func() {
		When("calling a route that requires authentication", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Delete(ctx, ownerID, "no-such-switch-"+uuid.NewString(), "not-a-valid-jwt")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		When("calling a route that allows anonymous callers", func() {
			var switchID string

			BeforeEach(func(ctx SpecContext) {
				switchID = "public-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, ownerID, switchID, "public")).To(Succeed())

				var err error
				resp, err = client.Get(ctx, ownerID, switchID, "not-a-valid-jwt")
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			})

			It("returns 401, not silently falling back to anonymous", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
