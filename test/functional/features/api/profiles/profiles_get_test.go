package profiles_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

type profileBody struct {
	Username        string  `json:"username"`
	Discoverable    *bool   `json:"discoverable"`
	DiscordUsername *string `json:"discord_username"`
	Bio             *string `json:"bio"`
	Links           *[]struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"links"`
	Avatar *struct {
		URL string `json:"url"`
	} `json:"avatar"`
	// Present only if the server wrongly leaked it - specs assert it stays
	// zero.
	UserID      string `json:"user_id"`
	StytchUser  string `json:"stytch_user_id"`
}

var _ = Describe("Getting a profile", func() {
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

	Context("given a discoverable profile exists", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:        username,
				Discoverable:    true,
				DiscordUsername: "alice_kb",
				Bio:             "keebs enjoyer",
				Links: []map[string]string{
					{"name": "Twitch", "url": "https://twitch.tv/alice"},
				},
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteProfile(ctx, ownerID, username)).To(Succeed())
		})

		Context("given the identifier is the owner's user id", func() {
			When("getting the profile", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the full profile without the IdP subject", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got profileBody
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Username).To(Equal(username))
					Expect(got.Bio).NotTo(BeNil())
					Expect(*got.Bio).To(Equal("keebs enjoyer"))
					Expect(got.Links).NotTo(BeNil())
					Expect(*got.Links).To(HaveLen(1))

					By("never leaking the IdP subject on the single-profile response")
					Expect(got.UserID).To(BeEmpty())
					Expect(got.StytchUser).To(BeEmpty())
				})
			})
		})

		Context("given the identifier is the username", func() {
			When("getting the profile anonymously", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, username, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("resolves the username and returns the same profile", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got profileBody
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.Username).To(Equal(username))
				})
			})
		})
	})

	Context("given a non-discoverable profile exists", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:     username,
				Discoverable: false,
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteProfile(ctx, ownerID, username)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the profile by its own user id", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the profile", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
				})
			})
		})

		Context("given the caller is a different user", func() {
			var otherToken string

			BeforeEach(func(ctx SpecContext) {
				var err error
				otherToken, err = api.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			When("getting the profile", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, username, otherToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing it exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the profile", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, username, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				})
			})
		})
	})

	Context("given no profile matches the identifier", func() {
		When("getting by a bogus user id", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, "user-nobody-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("getting by a bogus username", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, "ghost"+uuid.NewString()[:8], ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
})
