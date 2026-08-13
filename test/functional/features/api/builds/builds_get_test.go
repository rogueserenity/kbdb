package builds_test

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Getting a build", func() {
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

	seedBuild := func(ctx SpecContext, visibility string) string {
		id := visibility + "-build-" + uuid.NewString()
		Expect(db.SeedBuild(ctx, ownerID, id, keyboardID, visibility)).To(Succeed())
		return id
	}

	Context("given a public build", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			buildID = seedBuild(ctx, "public")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the build", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(buildID))
				})
			})
		})
	})

	Context("given a private build", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			buildID = seedBuild(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID)).To(Succeed())
		})

		Context("given the caller is the owner", func() {
			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the build", func() {
					By("returning 200 OK")
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					By("returning the build's id")
					var got struct {
						ID string `json:"id"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(buildID))
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

			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, token)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404, not 403, to avoid revealing the item exists", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given the caller is anonymous", func() {
			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 404", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the build does not exist", func() {
		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, "no-such-build-"+uuid.NewString(), ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
