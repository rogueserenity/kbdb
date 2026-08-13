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

var _ = Describe("Creating a build", func() {
	var (
		resp       *http.Response
		client     *api.BuildsClient
		ownerID    string
		ownerToken string
		keyboardID string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewBuildsClient()
		createdID = ""
		keyboardID = "build-fixture-keyboard-" + uuid.NewString()

		var err error
		ownerToken, err = api.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = api.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())

		// A build references a keyboard, so seed one directly rather than
		// through the API - keeps this spec focused on the build create
		// route itself.
		Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteBuild(ctx, ownerID, createdID)).To(Succeed())
		}
		Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
	})

	// captureCreatedID is called from BeforeEach, right after a successful
	// Create response is decoded - not from inside an It block after that
	// spec's own assertions. An assertion failing partway through an It
	// must not skip capturing this and leak the created build.
	captureCreatedID := func(r *http.Response) {
		var got struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&got)).To(Succeed())
		createdID = got.ID
	}

	Context("given the caller is the owner", func() {
		Context("given a valid request body", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"keyboard":"`+keyboardID+`","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the build", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning a server-generated id")
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given every open-vocabulary field has an approved lookup value", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"keyboard":"`+keyboardID+`","visibility":"private",`+
							`"stabs":{"name":"`+approvedStabilizer+`","mount_type":"`+approvedStabilizerMount+`"},`+
							`"case_mount_type":{"type":"`+approvedCaseMountType+`","durometer":"`+approvedDurometer+`"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the build", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given an open-vocabulary field has an unapproved value", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"keyboard":"`+keyboardID+`","visibility":"private","stabs":{"name":"NotApproved"}}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given required fields are missing", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken, `{}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})
	})

	Context("given the caller is anonymous", func() {
		Context("given an otherwise-valid body", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "",
						`{"keyboard":"`+keyboardID+`","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
			})
		})

		// Auth middleware runs before OpenAPI body validation, so a bad
		// token must win over a bad body.
		Context("given a body missing required fields", func() {
			When("creating a build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "", `{}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 401, not 400", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
				})
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

		When("creating a build in the owner's collection", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Create(ctx, ownerID, token,
					`{"keyboard":"`+keyboardID+`","visibility":"private"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404, not 403, to avoid revealing whose collection this is", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
