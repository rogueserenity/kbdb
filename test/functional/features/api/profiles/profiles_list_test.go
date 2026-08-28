package profiles_test

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// listRow is one /v1/profiles item. UserID is expected (the list -> detail
// chain, and to address the {userId} collection routes).
type listRow struct {
	Username        string  `json:"username"`
	UserID          string  `json:"user_id"`
	DiscordUsername *string `json:"discord_username"`
	Avatar          *struct {
		URL string `json:"url"`
	} `json:"avatar"`
	// not in the summary shape - specs assert absent
	Bio   *string `json:"bio"`
	Links *[]any  `json:"links"`
}

type listPage struct {
	Items      []listRow `json:"items"`
	NextCursor *string   `json:"next_cursor"`
}

var _ = Describe("Listing profiles", func() {
	var (
		resp       *http.Response
		client     *api.ProfilesClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewProfilesClient()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	decodePage := func(r *http.Response) listPage {
		var page listPage
		Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
		return page
	}

	usernames := func(page listPage) []string {
		out := make([]string, len(page.Items))
		for i, row := range page.Items {
			out[i] = row.Username
		}
		return out
	}

	Context("given a discoverable and a non-discoverable profile exist for the caller", func() {
		// The directory GSIs use a constant PK, so all test users' rows
		// share one index - specs filter by a unique prefix and assert
		// membership, not counts.
		var discoverableName, hiddenName string

		BeforeEach(func(ctx SpecContext) {
			discoverableName = "disc" + strings.ToLower(uuid.NewString()[:12])
			hiddenName = "hidden" + strings.ToLower(uuid.NewString()[:10])

			Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
				Username:        discoverableName,
				Discoverable:    true,
				DiscordUsername: "Disc_Handle",
				Bio:             "should not appear in the directory row",
				Links:           []map[string]string{{"name": "Site", "url": "https://example.com"}},
			})).To(Succeed())

			secondToken, err := api.SecondUserAuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			secondID, err := api.TokenSubject(secondToken)
			Expect(err).NotTo(HaveOccurred())
			Expect(db.SeedProfile(ctx, secondID, db.SeedProfileOptions{
				Username:     hiddenName,
				Discoverable: false,
			})).To(Succeed())
			DeferCleanup(func(cleanupCtx SpecContext) {
				Expect(db.DeleteProfile(cleanupCtx, secondID, hiddenName)).To(Succeed())
			})
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteProfile(ctx, ownerID, discoverableName)).To(Succeed())
		})

		Context("given the username prefix filter matches only the discoverable one", func() {
			When("listing the directory", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, "", api.ListProfilesQuery{Limit: -1, Username: discoverableName})
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the discoverable profile, with user_id but no bio or links", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					page := decodePage(resp)
					Expect(usernames(page)).To(ConsistOf(discoverableName))

					row := page.Items[0]
					By("carrying the owner's user id for the list -> detail chain")
					Expect(row.UserID).To(Equal(ownerID))
					By("omitting bio and links from the summary shape")
					Expect(row.Bio).To(BeNil())
					Expect(row.Links).To(BeNil())
				})
			})
		})

		Context("given no filter is supplied", func() {
			When("listing the directory", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, "", api.ListProfilesQuery{Limit: -1})
					Expect(err).NotTo(HaveOccurred())
				})

				It("includes the discoverable profile and never the non-discoverable one", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					names := usernames(decodePage(resp))
					Expect(names).To(ContainElement(discoverableName))
					Expect(names).NotTo(ContainElement(hiddenName))
				})
			})
		})

		Context("given the discord_username prefix filter", func() {
			When("listing the directory with a lowercased prefix of a mixed-case handle", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, "", api.ListProfilesQuery{Limit: -1, DiscordUsername: "disc_h"})
					Expect(err).NotTo(HaveOccurred())
				})

				It("matches via the discord index", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					Expect(usernames(decodePage(resp))).To(ContainElement(discoverableName))
				})
			})
		})
	})

	Context("given both prefix filters are supplied", func() {
		When("listing the directory", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, "", api.ListProfilesQuery{Limit: -1, Username: "a", DiscordUsername: "b"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	Context("given more discoverable profiles than the page limit share a username prefix", func() {
		var prefix string
		var names []string

		BeforeEach(func(ctx SpecContext) {
			prefix = "page" + strings.ToLower(uuid.NewString()[:10])
			names = []string{prefix + "a", prefix + "b", prefix + "c"}

			secondToken, err := api.SecondUserAuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			secondID, err := api.TokenSubject(secondToken)
			Expect(err).NotTo(HaveOccurred())

			// One profile per user and only two real identities, so the
			// third is a synthetic owner id - fine, this spec only reads
			// the directory anonymously.
			owners := []string{ownerID, secondID, "user-page-" + uuid.NewString()}
			for i, name := range names {
				Expect(db.SeedProfile(ctx, owners[i], db.SeedProfileOptions{
					Username:     name,
					Discoverable: true,
				})).To(Succeed())
				owner := owners[i]
				uname := name
				DeferCleanup(func(cleanupCtx SpecContext) {
					Expect(db.DeleteProfile(cleanupCtx, owner, uname)).To(Succeed())
				})
			}
		})

		When("paging through the prefix with limit 2", func() {
			It("round-trips next_cursor to return the remaining profiles", func(ctx SpecContext) {
				first, err := client.List(ctx, "", api.ListProfilesQuery{Limit: 2, Username: prefix})
				Expect(err).NotTo(HaveOccurred())
				Expect(first.StatusCode).To(Equal(http.StatusOK))

				page1 := decodePage(first)
				Expect(page1.Items).To(HaveLen(2))
				Expect(page1.NextCursor).NotTo(BeNil())
				Expect(*page1.NextCursor).NotTo(BeEmpty())

				second, err := client.List(ctx, "", api.ListProfilesQuery{
					Limit: 2, Username: prefix, Cursor: *page1.NextCursor,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(second.StatusCode).To(Equal(http.StatusOK))

				page2 := decodePage(second)
				all := append(usernames(page1), usernames(page2)...)
				Expect(all).To(ConsistOf(names))

				By("rejecting that cursor when reused with a different filter")
				bad, err := client.List(ctx, "", api.ListProfilesQuery{
					Limit: 2, DiscordUsername: "x", Cursor: *page1.NextCursor,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(bad.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})
	})

	Context("given no bearer token", func() {
		When("listing the directory anonymously", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, "", api.ListProfilesQuery{Limit: -1})
				Expect(err).NotTo(HaveOccurred())
			})

			It("still returns 200", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
			})
		})
	})
})
