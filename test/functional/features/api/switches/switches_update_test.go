package switches_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Updating a switch", func() {
	var (
		resp       *http.Response
		client     *api.SwitchesClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given a private switch owned by the caller", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = "update-switch-" + uuid.NewString()
			Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			Context("given a valid request body", func() {
				When("updating the switch", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.Update(ctx, ownerID, switchID, ownerToken,
							`{"brand":"Gateron","name":"Yellow2","type":"Linear","visibility":"private"}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns 200 with the updated switch", func() {
						By("returning 200 OK")
						Expect(resp.StatusCode).To(Equal(http.StatusOK))

						By("returning the switch's id and updated name")
						var got struct {
							ID   string `json:"id"`
							Name string `json:"name"`
						}
						Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
						Expect(got.ID).To(Equal(switchID))
						Expect(got.Name).To(Equal("Yellow2"))
					})
				})
			})

			Context("given an open-vocabulary field has an unapproved value", func() {
				When("updating the switch", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.Update(ctx, ownerID, switchID, ownerToken,
							`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private","material":{"stem":"NotApproved"}}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns 400 with a problem+json body", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
					})
				})
			})

			Context("given required fields are missing", func() {
				When("updating the switch", func() {
					BeforeEach(func(ctx SpecContext) {
						var err error
						resp, err = client.Update(ctx, ownerID, switchID, ownerToken, `{"brand":"Gateron"}`)
						Expect(err).NotTo(HaveOccurred())
					})

					It("returns 400 with a problem+json body", func() {
						Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
						Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
					})
				})
			})
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("updating the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Update(ctx, ownerID, switchID, token,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("updating the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Update(ctx, ownerID, switchID, "",
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})
	})

	Context("given the switch does not exist", func() {
		Context("given the caller is the owner", func() {
			When("updating the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Update(ctx, ownerID, "no-such-switch-"+uuid.NewString(), ownerToken,
						`{"brand":"Gateron","name":"Yellow","type":"Linear","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not idempotent like delete", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})
})
