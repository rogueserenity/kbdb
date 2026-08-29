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

var _ = Describe("Updating a profile over MCP", func() {
	var (
		client   *api.MCPClient
		result   *sdkmcp.CallToolResult
		err      error
		ownerID  string
		username string
	)

	BeforeEach(func(ctx SpecContext) {
		result = nil
		err = nil
		username = "u" + uuid.NewString()[:8]

		token, tokenErr := api.AuthToken(ctx)
		Expect(tokenErr).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(token)
		Expect(err).NotTo(HaveOccurred())

		client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
	})

	Context("given the caller has a profile", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:        username,
				Discoverable:    true,
				Bio:             "original bio",
				DiscordUsername: "alice_kb",
				Links: []map[string]string{
					{"name": "Twitch", "url": "https://twitch.tv/alice"},
				},
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			_ = db.DeleteProfile(ctx, ownerID, username)
		})

		Context("given a full replace that changes the bio and omits links", func() {
			When("update_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":     username,
						"discoverable": true,
						"bio":          "brand new bio",
					})
				})

				It("changes the bio and clears the links", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeGetProfileOutput(result)
					Expect(out.Profile.Bio).NotTo(BeNil())
					Expect(*out.Profile.Bio).To(Equal("brand new bio"))
					Expect(out.Profile.Links).To(BeEmpty())

					read, readErr := client.CallTool(ctx, "get_profile", map[string]any{"identifier": ownerID})
					Expect(readErr).NotTo(HaveOccurred())
					Expect(decodeGetProfileOutput(read).Profile.Links).To(BeEmpty())
				})
			})
		})

		Context("given a rename to an unclaimed username", func() {
			var newUsername string

			BeforeEach(func() {
				newUsername = "u" + uuid.NewString()[:8]
			})

			AfterEach(func(ctx SpecContext) {
				_ = db.DeleteProfile(ctx, ownerID, newUsername)
			})

			When("update_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":     newUsername,
						"discoverable": true,
					})
				})

				It("moves the username claim", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					byNew, readErr := client.CallTool(ctx, "get_profile", map[string]any{"identifier": newUsername})
					Expect(readErr).NotTo(HaveOccurred())
					Expect(byNew.IsError).To(BeFalse())

					byOld, readErr := client.CallTool(ctx, "get_profile", map[string]any{"identifier": username})
					Expect(readErr).NotTo(HaveOccurred())
					Expect(byOld.IsError).To(BeTrue())
				})
			})
		})

		Context("given the same username", func() {
			When("update_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":     username,
						"discoverable": true,
						"bio":          "no rename",
					})
				})

				It("succeeds with no self-conflict", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
				})
			})
		})

		Context("given a blank discord_username", func() {
			When("update_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":         username,
						"discoverable":     true,
						"discord_username": "   ",
					})
				})

				It("clears discord_username instead of leaving it empty", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeGetProfileOutput(result).Profile.DiscordUsername).To(BeNil())
				})
			})
		})

		Context("given a malformed discord_username", func() {
			When("update_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":         username,
						"discoverable":     true,
						"discord_username": "na@me",
					})
				})

				It("returns a tool error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given the new username is taken by another user", func() {
			var (
				otherID   string
				otherUser string
			)

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())
				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				otherUser = "u" + uuid.NewString()[:8]
				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: otherUser, Discoverable: true,
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteProfile(ctx, otherID, otherUser)).To(Succeed())
			})

			When("update_profile is called with that username", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "update_profile", map[string]any{
						"username":     otherUser,
						"discoverable": true,
					})
				})

				It("returns a tool error", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})
	})

	Context("given the caller has no profile", func() {
		When("update_profile is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "update_profile", map[string]any{"username": username})
			})

			It("returns a tool error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
