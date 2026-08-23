package builds_test

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Listing builds", func() {
	var (
		resp       *http.Response
		client     *api.BuildsClient
		ownerID    string
		ownerToken string
		keyboardID string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewBuildsClient()
		keyboardID = "build-fixture-keyboard-" + uuid.NewString()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())

		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	type listItem struct {
		ID       string `json:"id"`
		Keyboard *struct {
			Brand string `json:"brand"`
			Name  string `json:"name"`
		} `json:"keyboard"`
		TotalCost *float64 `json:"total_cost"`
	}

	decodeItems := func(r *http.Response) []listItem {
		var page struct {
			Items []listItem `json:"items"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
		return page.Items
	}

	itemIDs := func(items []listItem) []string {
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
		}
		return ids
	}

	Context("given the owner has no builds", func() {
		When("listing builds", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, -1)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns an empty page", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(decodeItems(resp)).To(BeEmpty())
			})
		})
	})

	Context("given the owner has builds at every visibility tier", func() {
		var publicID, authenticatedID, privateID string

		BeforeEach(func(ctx SpecContext) {
			publicID = "public-build-" + uuid.NewString()
			authenticatedID = "authenticated-build-" + uuid.NewString()
			privateID = "private-build-" + uuid.NewString()

			Expect(db.SeedBuild(ctx, ownerID, publicID, keyboardID, "public")).To(Succeed())
			Expect(db.SeedBuild(ctx, ownerID, authenticatedID, keyboardID, "authenticated")).To(Succeed())
			Expect(db.SeedBuild(ctx, ownerID, privateID, keyboardID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, publicID, keyboardID)).To(Succeed())
			Expect(db.DeleteBuild(ctx, ownerID, authenticatedID, keyboardID)).To(Succeed())
			Expect(db.DeleteBuild(ctx, ownerID, privateID, keyboardID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("listing builds", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns builds at every visibility tier, each with the denormalized keyboard brand/name", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					items := decodeItems(resp)
					By("including all three seeded builds")
					Expect(itemIDs(items)).To(ContainElements(publicID, authenticatedID, privateID))

					By("denormalizing the referenced keyboard's brand/name onto every item")
					for _, item := range items {
						Expect(item.Keyboard).NotTo(BeNil())
						Expect(item.Keyboard.Brand).To(Equal("Keychron"))
						Expect(item.Keyboard.Name).To(Equal("Q1"))
					}
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("listing builds", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the public build", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(decodeItems(resp))
					Expect(ids).To(ContainElement(publicID))
					Expect(ids).NotTo(ContainElement(authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
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

			When("listing builds", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the public and authenticated builds, but not the private one", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(decodeItems(resp))
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
			})
		})
	})

	Context("given the owner has a build with priced components", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			buildID = "priced-build-" + uuid.NewString()
			Expect(db.SeedBuildWithStabs(ctx, ownerID, buildID, keyboardID, "public")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID, keyboardID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("listing builds", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("includes total_cost derived from the keyboard's price and the stabs' price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					items := decodeItems(resp)
					idx := slices.IndexFunc(items, func(i listItem) bool { return i.ID == buildID })
					Expect(idx).To(BeNumerically(">=", 0))
					Expect(items[idx].TotalCost).NotTo(BeNil())
					Expect(*items[idx].TotalCost).To(Equal(329.99 + 12.5))
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

			When("listing builds", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("omits total_cost even though the build itself is visible", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					items := decodeItems(resp)
					idx := slices.IndexFunc(items, func(i listItem) bool { return i.ID == buildID })
					Expect(idx).To(BeNumerically(">=", 0))
					Expect(items[idx].TotalCost).To(BeNil())
				})
			})
		})
	})

	Context("given the owner has more builds than fit under a small limit, each referencing the keyboard", func() {
		var buildIDs []string

		BeforeEach(func(ctx SpecContext) {
			buildIDs = nil
			for range 5 {
				id := "paged-build-" + uuid.NewString()
				Expect(db.SeedBuild(ctx, ownerID, id, keyboardID, "public")).To(Succeed())
				buildIDs = append(buildIDs, id)
			}
		})

		AfterEach(func(ctx SpecContext) {
			for _, id := range buildIDs {
				Expect(db.DeleteBuild(ctx, ownerID, id, keyboardID)).To(Succeed())
			}
		})

		When("listing builds with a limit smaller than the build count", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.List(ctx, ownerID, ownerToken, 3)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns a full page of builds, not markers filtered down to a partial page", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(decodeItems(resp)).To(HaveLen(3))
			})
		})
	})

	DescribeTable("given an invalid limit",
		func(ctx SpecContext, limit int) {
			var err error
			resp, err = client.List(ctx, ownerID, ownerToken, limit)
			Expect(err).NotTo(HaveOccurred())

			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
		},
		Entry("below the minimum", 0),
		Entry("above the maximum", 101),
	)
})
