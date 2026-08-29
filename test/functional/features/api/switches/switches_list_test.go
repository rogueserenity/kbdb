package switches_test

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Listing switches", func() {
	var (
		resp       *http.Response
		client     *api.SwitchesClient
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewSwitchesClient()

		// Derived from a freshly minted token, not a fixed fixture subject:
		// in CI, AuthToken mints a real subject from the WorkOS emulator
		// rather than a fixed fixture subject string, so the owner used
		// to seed fixture data below must match whatever subject this
		// environment's token actually carries.
		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	itemIDs := func(r *http.Response) []string {
		var page struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&page)).To(Succeed())
		ids := make([]string, len(page.Items))
		for i, item := range page.Items {
			ids[i] = item.ID
		}
		return ids
	}

	Context("given the owner has switches at every visibility tier", func() {
		var publicID, authenticatedID, privateID string

		BeforeEach(func(ctx SpecContext) {
			// Fresh IDs per spec run, not fixed literals: AuthToken is a
			// shared, real emulator identity in CI (provisioning one per
			// spec isn't practical there), so specs sharing an identity
			// must namespace their own data to stay collision-proof under
			// concurrent/out-of-order runs.
			publicID = "public-switch-" + uuid.NewString()
			authenticatedID = "authenticated-switch-" + uuid.NewString()
			privateID = "private-switch-" + uuid.NewString()

			Expect(db.SeedSwitch(ctx, ownerID, publicID, "public")).To(Succeed())
			Expect(db.SeedSwitch(ctx, ownerID, authenticatedID, "authenticated")).To(Succeed())
			Expect(db.SeedSwitch(ctx, ownerID, privateID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteSwitch(ctx, ownerID, publicID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, authenticatedID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, privateID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, ownerToken, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns switches at every visibility tier", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					body, err := io.ReadAll(resp.Body)
					Expect(err).NotTo(HaveOccurred())

					var page struct {
						Items []struct {
							ID string `json:"id"`
						} `json:"items"`
					}
					Expect(json.Unmarshal(body, &page)).To(Succeed())
					ids := make([]string, len(page.Items))
					for i, item := range page.Items {
						ids[i] = item.ID
					}

					By("including all three seeded switches")
					Expect(ids).To(ContainElements(publicID, authenticatedID, privateID))

					By("omitting the image key on image-less switches rather than emitting it as null")
					var raw struct {
						Items []map[string]json.RawMessage `json:"items"`
					}
					Expect(json.Unmarshal(body, &raw)).To(Succeed())
					for _, item := range raw.Items {
						Expect(item).NotTo(HaveKey("image"))
					}
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, "", -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns only the public switch", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(resp)
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

			When("listing switches", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.List(ctx, ownerID, token, -1)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the public and authenticated switches, but not the private one", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))
					ids := itemIDs(resp)
					Expect(ids).To(ContainElements(publicID, authenticatedID))
					Expect(ids).NotTo(ContainElement(privateID))
				})
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

	Context("given a non-numeric limit", func() {
		When("listing switches", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.ListWithRawLimit(ctx, ownerID, ownerToken, "abc")
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
