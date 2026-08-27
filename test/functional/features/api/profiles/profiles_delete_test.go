package profiles_test

import (
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Deleting a profile", func() {
	var (
		resp       *http.Response
		client     *api.ProfilesClient
		ownerID    string
		ownerToken string
		username   string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewProfilesClient()
		username = "u" + uuid.NewString()[:8]

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given the caller has a profile", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:     username,
				Discoverable: true,
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			// The delete cases remove the profile; clean up best-effort in
			// case an assertion failed before the delete landed.
			_ = db.DeleteProfile(ctx, ownerID, username)
		})

		When("deleting the profile", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Delete(ctx, ownerID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 204, the profile is gone, and the username is free again", func(ctx SpecContext) {
				By("returning 204")
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

				By("no longer resolving the profile by id")
				got, err := client.Get(ctx, ownerID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
				Expect(got.StatusCode).To(Equal(http.StatusNotFound))

				By("letting another user claim the freed username")
				otherToken, err := api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				otherID, err := api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				created, err := client.Create(ctx, otherID, otherToken,
					`{"username": "`+username+`", "discoverable": true}`)
				Expect(err).NotTo(HaveOccurred())
				Expect(created.StatusCode).To(Equal(http.StatusCreated))

				DeferCleanup(func(ctx SpecContext) {
					Expect(db.DeleteProfile(ctx, otherID, username)).To(Succeed())
				})
			})
		})
	})

	Context("given the caller has no profile", func() {
		When("deleting the profile", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Delete(ctx, ownerID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 204 (idempotent)", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			})
		})
	})

	Context("given the path identifier is not the caller", func() {
		When("deleting the profile", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Delete(ctx, "user-someone-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404, not 403, to avoid revealing whose profile it is", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
