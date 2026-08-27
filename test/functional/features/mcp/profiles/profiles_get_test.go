package profiles_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a profile over MCP", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		ownerID  string
		username string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		username = "u" + uuid.NewString()[:8]
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given a discoverable profile exists", func() {
			BeforeEach(func(ctx SpecContext) {
				Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
					Username:        username,
					Discoverable:    true,
					DiscordUsername: "alice_kb",
					Bio:             "keebs",
					Links:           []map[string]string{{"name": "Twitch", "url": "https://twitch.tv/alice"}},
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteProfile(ctx, ownerID, username)).To(Succeed())
			})

			When("get_profile is called with the user id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_profile", map[string]any{"identifier": ownerID})
				})

				It("returns the full profile", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetProfileOutput(result)
					Expect(out.Profile.Username).To(Equal(username))
					Expect(out.Profile.Bio).NotTo(BeNil())
					Expect(*out.Profile.Bio).To(Equal("keebs"))
					Expect(out.Profile.Links).To(HaveLen(1))
					Expect(out.Profile.HasAvatar).To(BeFalse())
				})
			})

			When("get_profile is called with the username", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_profile", map[string]any{"identifier": username})
				})

				It("resolves the username and returns the profile", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeGetProfileOutput(result).Profile.Username).To(Equal(username))
				})
			})
		})

		Context("given a non-discoverable profile owned by another user", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())
				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username:     username,
					Discoverable: false,
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteProfile(ctx, otherID, username)).To(Succeed())
			})

			When("get_profile is called with that user's id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_profile", map[string]any{"identifier": otherID})
				})

				It("is indistinguishable from the profile not existing", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given no profile matches the identifier", func() {
			When("get_profile is called with a bogus username", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "get_profile", map[string]any{
						"identifier": "ghost" + uuid.NewString()[:8],
					})
				})

				It("returns a not-found tool error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})
	})
})
