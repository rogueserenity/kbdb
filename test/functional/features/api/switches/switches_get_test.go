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

var _ = Describe("Getting a switch", func() {
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

	seedSwitch := func(ctx SpecContext, visibility string) string {
		id := visibility + "-switch-" + uuid.NewString()
		Expect(db.SeedSwitch(ctx, ownerID, id, visibility)).To(Succeed())
		return id
	}

	Context("given a public switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "public")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))
				})
			})
		})
	})

	Context("given an authenticated-only switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "authenticated")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is a different authenticated user", func() {
			var token string

			BeforeEach(func(ctx SpecContext) {
				var err error
				token, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given a private switch", func() {
		var switchID string

		BeforeEach(func(ctx SpecContext) {
			switchID = seedSwitch(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the switch", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the switch's id")
					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(switchID))
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

			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the switch", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, switchID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the switch does not exist", func() {
		When("getting the switch", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, "no-such-switch-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
