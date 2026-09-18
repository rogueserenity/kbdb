package profiles_test

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

// getAvatarURL GETs the profile and returns its avatar.url, failing the
// spec if the profile has no avatar.
func getAvatarURL(ctx SpecContext, client *api.ProfilesClient, identifier, token string) string {
	resp, err := client.Get(ctx, identifier, token)
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = resp.Body.Close() }()
	Expect(resp.StatusCode).To(Equal(http.StatusOK))

	var got profileBody
	Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
	Expect(got.Avatar).NotTo(BeNil())
	return got.Avatar.URL
}

var _ = Describe("Presigned avatar GET URL caching for a profile", func() {
	var (
		client     *api.ProfilesClient
		ownerID    string
		ownerToken string
		username   string
	)

	BeforeEach(func(ctx SpecContext) {
		client = api.NewProfilesClient()
		username = "u" + uuid.NewString()[:8]

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())

		Expect(db.SeedProfile(ctx, ownerID, db.SeedProfileOptions{
			Username:     username,
			Discoverable: true,
		})).To(Succeed())

		setResp, err := client.SetImage(ctx, ownerID, ownerToken, `{"content_type":"`+approvedImageContentType+`"}`)
		Expect(err).NotTo(HaveOccurred())
		Expect(setResp.StatusCode).To(Equal(http.StatusCreated))

		var created struct {
			UploadURL string `json:"upload_url"`
		}
		Expect(json.NewDecoder(setResp.Body).Decode(&created)).To(Succeed())

		putResp, err := api.DoPresigned(ctx, http.MethodPut, created.UploadURL, approvedImageContentType, bytes.NewReader([]byte("fake-avatar-bytes")))
		Expect(err).NotTo(HaveOccurred())
		Expect(putResp.StatusCode).To(Equal(http.StatusOK))
	})

	AfterEach(func(ctx SpecContext) {
		Expect(db.DeleteProfile(ctx, ownerID, username)).To(Succeed())
	})

	Context("given the profile's avatar path has not changed", func() {
		When("getting the profile twice in a row", func() {
			It("returns the exact same presigned avatar URL both times", func(ctx SpecContext) {
				firstURL := getAvatarURL(ctx, client, ownerID, ownerToken)
				secondURL := getAvatarURL(ctx, client, ownerID, ownerToken)

				Expect(secondURL).To(Equal(firstURL))
			})
		})
	})

	// Invalidation-on-replace (a fresh presign happens, not a stale cached
	// URL) is covered at the unit level instead - see
	// TestSetAvatarPath_AlsoClearsCachedGetURL and friends in
	// internal/repository/dynamo/profile_test.go. A functional assertion
	// that the *URL string* differs after a replace is unreliable here:
	// AWS SigV4 presigned URLs for the same S3 key are byte-identical when
	// minted within the same UTC second (no nonce in the signature), and
	// this spec's replace-then-GET happens fast enough in CI to
	// occasionally land in that window even when invalidation works
	// correctly - a false failure, not a real regression signal.
})
