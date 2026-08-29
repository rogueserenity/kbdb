package profiles_test

import (
	"strings"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Creating a profile over MCP", func() {
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

	Context("given the caller has no profile", func() {
		AfterEach(func(ctx SpecContext) {
			_ = db.DeleteProfile(ctx, ownerID, username)
		})

		Context("given valid input", func() {
			When("create_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_profile", map[string]any{
						"username":     username,
						"discoverable": true,
						"bio":          "keebs",
					})
				})

				It("creates the profile and it is then readable", func(ctx SpecContext) {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeGetProfileOutput(result).Profile.Username).To(Equal(username))

					read, readErr := client.CallTool(ctx, "get_profile", map[string]any{"identifier": ownerID})
					Expect(readErr).NotTo(HaveOccurred())
					Expect(read.IsError).To(BeFalse())
					Expect(decodeGetProfileOutput(read).Profile.Username).To(Equal(username))
				})
			})
		})

		Context("given a blank discord_username", func() {
			When("create_profile is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_profile", map[string]any{
						"username":         username,
						"discoverable":     true,
						"discord_username": "   ",
					})
				})

				It("creates the profile with discord_username absent, not empty", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeGetProfileOutput(result).Profile.DiscordUsername).To(BeNil())
				})
			})
		})

		Context("given a username already claimed by another user", func() {
			var otherID string

			BeforeEach(func(ctx SpecContext) {
				otherToken, tokenErr := api.SecondUserAuthToken(ctx)
				Expect(tokenErr).NotTo(HaveOccurred())
				otherID, err = api.TokenSubject(otherToken)
				Expect(err).NotTo(HaveOccurred())

				Expect(db.SeedProfile(ctx, otherID, db.SeedProfileOptions{
					Username: username, Discoverable: true,
				})).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteProfile(ctx, otherID, username)).To(Succeed())
			})

			When("create_profile is called with that username", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_profile", map[string]any{"username": username})
				})

				It("returns a tool error naming the taken username", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		DescribeTable("given the input fails validation",
			func(ctx SpecContext, args map[string]any) {
				result, err = client.CallTool(ctx, "create_profile", args)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			},
			Entry("username fails the pattern", map[string]any{"username": "AB"}),
			Entry(`username starts with "user-"`, map[string]any{"username": "user-alice"}),
			Entry("a link url is http not https", map[string]any{
				"username": "aaa",
				"links":    []map[string]any{{"name": "s", "url": "http://x.example"}},
			}),
			Entry("discord_username over 32", map[string]any{
				"username": "aaa", "discord_username": strings.Repeat("x", 33),
			}),
			Entry("bio over 500", map[string]any{
				"username": "aaa", "bio": strings.Repeat("x", 501),
			}),
		)
	})

	Context("given the caller already has a profile", func() {
		BeforeEach(func(ctx SpecContext) {
			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username: username, Discoverable: false,
			})).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteProfile(ctx, ownerID, username)).To(Succeed())
		})

		When("create_profile is called again", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_profile", map[string]any{
					"username": "other" + uuid.NewString()[:6],
				})
			})

			It("returns a tool error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeTrue())
			})
		})
	})
})
