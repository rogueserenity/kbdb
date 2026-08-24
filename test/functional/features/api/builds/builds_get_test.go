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
			Expect(db.DeleteBuild(ctx, ownerID, buildID, keyboardID)).To(Succeed())
		})

		Context("given the caller is anonymous", func() {
			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, "")
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the build without total_cost", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						ID        string   `json:"id"`
						TotalCost *float64 `json:"total_cost"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.ID).To(Equal(buildID))
					Expect(got.TotalCost).To(BeNil())
				})
			})
		})

		Context("given the caller is the owner", func() {
			When("getting the build", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns the build with total_cost derived from the referenced keyboard's price", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusOK))

					var got struct {
						TotalCost *float64 `json:"total_cost"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.TotalCost).NotTo(BeNil())
					Expect(*got.TotalCost).To(Equal(329.99))
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
			Expect(db.DeleteBuild(ctx, ownerID, buildID, keyboardID)).To(Succeed())
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

	Context("given a build referencing a keyboard that still exists", func() {
		var buildID string

		BeforeEach(func(ctx SpecContext) {
			buildID = seedBuild(ctx, "private")
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID, keyboardID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("denormalizes the referenced keyboard's brand and name", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Keyboard *struct {
						ID    string `json:"id"`
						Brand string `json:"brand"`
						Name  string `json:"name"`
					} `json:"keyboard"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Keyboard).NotTo(BeNil())
				Expect(got.Keyboard.ID).To(Equal(keyboardID))
				Expect(got.Keyboard.Brand).To(Equal("Keychron"))
				Expect(got.Keyboard.Name).To(Equal("Q1"))
			})
		})
	})

	Context("given a build referencing a keyboard that no longer exists", func() {
		var (
			buildID     string
			deletedKbID string
		)

		BeforeEach(func(ctx SpecContext) {
			deletedKbID = "deleted-keyboard-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedBuild(ctx, ownerID, buildID, deletedKbID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID, deletedKbID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("omits the keyboard field rather than failing", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Keyboard *struct{} `json:"keyboard"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Keyboard).To(BeNil())
			})
		})
	})

	Context("given a build referencing a switch that still exists", func() {
		var (
			buildID  string
			switchID string
		)

		BeforeEach(func(ctx SpecContext) {
			switchID = "build-fixture-switch-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
			Expect(db.SeedBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, switchID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, switchID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("denormalizes the referenced switch's brand and name", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Switches []struct {
						Switch *struct {
							ID    string `json:"id"`
							Brand string `json:"brand"`
							Name  string `json:"name"`
						} `json:"switch"`
						Count int `json:"count"`
					} `json:"switches"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Switches).To(HaveLen(1))
				Expect(got.Switches[0].Switch).NotTo(BeNil())
				Expect(got.Switches[0].Switch.ID).To(Equal(switchID))
				Expect(got.Switches[0].Switch.Brand).To(Equal("Gateron"))
				Expect(got.Switches[0].Switch.Name).To(Equal("Yellow"))
				Expect(got.Switches[0].Count).To(Equal(1))
			})
		})
	})

	Context("given a build referencing a switch that no longer exists", func() {
		var (
			buildID  string
			switchID string
		)

		BeforeEach(func(ctx SpecContext) {
			switchID = "deleted-switch-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, switchID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, switchID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("keeps the entry's count but omits the switch field", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Switches []struct {
						Switch *struct{} `json:"switch"`
						Count  int       `json:"count"`
					} `json:"switches"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Switches).To(HaveLen(1))
				Expect(got.Switches[0].Switch).To(BeNil())
				Expect(got.Switches[0].Count).To(Equal(1))
			})
		})
	})

	Context("given a build referencing a keycap kit that still exists", func() {
		var (
			buildID     string
			keycapSetID string
			kitID       string
		)

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = "build-fixture-keycap-set-" + uuid.NewString()
			kitID = "kit-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedKeycapSetWithKit(ctx, ownerID, keycapSetID, kitID, "private")).To(Succeed())
			Expect(db.SeedBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID)).To(Succeed())
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("denormalizes the referenced keycap set and kit name", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					KeycapKits []struct {
						KeycapSet *struct {
							ID    string `json:"id"`
							Brand string `json:"brand"`
							Name  string `json:"name"`
						} `json:"keycap_set"`
						KitID   string  `json:"kit_id"`
						KitName *string `json:"kit_name"`
					} `json:"keycap_kits"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.KeycapKits).To(HaveLen(1))
				Expect(got.KeycapKits[0].KeycapSet).NotTo(BeNil())
				Expect(got.KeycapKits[0].KeycapSet.ID).To(Equal(keycapSetID))
				Expect(got.KeycapKits[0].KeycapSet.Brand).To(Equal("GMK"))
				Expect(got.KeycapKits[0].KeycapSet.Name).To(Equal("Laser"))
				Expect(got.KeycapKits[0].KitID).To(Equal(kitID))
				Expect(got.KeycapKits[0].KitName).NotTo(BeNil())
				Expect(*got.KeycapKits[0].KitName).To(Equal("Base"))
			})
		})
	})

	Context("given a build referencing a keycap set that no longer exists", func() {
		var (
			buildID     string
			keycapSetID string
			kitID       string
		)

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = "deleted-keycap-set-" + uuid.NewString()
			kitID = "kit-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("keeps the entry's kit_id but omits keycap_set and kit_name", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					KeycapKits []struct {
						KeycapSet *struct{} `json:"keycap_set"`
						KitID     string    `json:"kit_id"`
						KitName   *string   `json:"kit_name"`
					} `json:"keycap_kits"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.KeycapKits).To(HaveLen(1))
				Expect(got.KeycapKits[0].KeycapSet).To(BeNil())
				Expect(got.KeycapKits[0].KitID).To(Equal(kitID))
				Expect(got.KeycapKits[0].KitName).To(BeNil())
			})
		})
	})

	Context("given a build referencing a keycap set that exists but no longer has the referenced kit", func() {
		var (
			buildID     string
			keycapSetID string
			kitID       string
		)

		BeforeEach(func(ctx SpecContext) {
			keycapSetID = "build-fixture-keycap-set-" + uuid.NewString()
			kitID = "removed-kit-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			// The set exists, but its kit list doesn't include kitID - as if
			// the kit itself had been deleted independently of its set.
			Expect(db.SeedKeycapSetWithKit(ctx, ownerID, keycapSetID, "some-other-kit-"+uuid.NewString(), "private")).To(Succeed())
			Expect(db.SeedBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithKeycapKitAndKeyboard(ctx, ownerID, buildID, keyboardID, keycapSetID, kitID)).To(Succeed())
			Expect(db.DeleteKeycapSet(ctx, ownerID, keycapSetID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("keeps the entry's kit_id but omits keycap_set and kit_name", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					KeycapKits []struct {
						KeycapSet *struct{} `json:"keycap_set"`
						KitID     string    `json:"kit_id"`
						KitName   *string   `json:"kit_name"`
					} `json:"keycap_kits"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.KeycapKits).To(HaveLen(1))
				Expect(got.KeycapKits[0].KeycapSet).To(BeNil())
				Expect(got.KeycapKits[0].KitID).To(Equal(kitID))
				Expect(got.KeycapKits[0].KitName).To(BeNil())
			})
		})
	})

	Context("given a build referencing a keyboard that has an image", func() {
		var (
			buildID   string
			imageKbID string
			imageID   string
		)

		BeforeEach(func(ctx SpecContext) {
			imageKbID = "build-fixture-keyboard-with-image-" + uuid.NewString()
			imageID = "img-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedKeyboardWithImage(ctx, ownerID, imageKbID, imageID, "private")).To(Succeed())
			Expect(db.SeedBuild(ctx, ownerID, buildID, imageKbID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuild(ctx, ownerID, buildID, imageKbID)).To(Succeed())
			Expect(db.DeleteKeyboard(ctx, ownerID, imageKbID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("includes a presigned image_url for the keyboard's first image", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Keyboard *struct {
						ImageURL *string `json:"image_url"`
					} `json:"keyboard"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Keyboard).NotTo(BeNil())
				Expect(got.Keyboard.ImageURL).NotTo(BeNil())
				Expect(*got.Keyboard.ImageURL).NotTo(BeEmpty())
			})
		})
	})

	Context("given a build referencing a switch that has an image", func() {
		var (
			buildID       string
			imageSwitchID string
		)

		BeforeEach(func(ctx SpecContext) {
			imageSwitchID = "build-fixture-switch-with-image-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedSwitchWithImage(ctx, ownerID, imageSwitchID, "private")).To(Succeed())
			Expect(db.SeedBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, imageSwitchID, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithSwitchAndKeyboard(ctx, ownerID, buildID, keyboardID, imageSwitchID)).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, imageSwitchID)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("includes a presigned image_url for the switch", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Switches []struct {
						Switch *struct {
							ImageURL *string `json:"image_url"`
						} `json:"switch"`
					} `json:"switches"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Switches).To(HaveLen(1))
				Expect(got.Switches[0].Switch).NotTo(BeNil())
				Expect(got.Switches[0].Switch.ImageURL).NotTo(BeNil())
				Expect(*got.Switches[0].Switch.ImageURL).NotTo(BeEmpty())
			})
		})
	})

	Context("given a build referencing two switches, one deleted and one still existing", func() {
		var (
			buildID          string
			resolvableSwitch string
			deletedSwitch    string
		)

		BeforeEach(func(ctx SpecContext) {
			resolvableSwitch = "build-fixture-switch-" + uuid.NewString()
			deletedSwitch = "deleted-switch-" + uuid.NewString()
			buildID = "private-build-" + uuid.NewString()
			Expect(db.SeedSwitch(ctx, ownerID, resolvableSwitch, "private")).To(Succeed())
			Expect(db.SeedBuildWithSwitchesAndKeyboard(ctx, ownerID, buildID, keyboardID,
				[]string{deletedSwitch, resolvableSwitch}, "private")).To(Succeed())
		})

		AfterEach(func(ctx SpecContext) {
			Expect(db.DeleteBuildWithSwitchesAndKeyboard(ctx, ownerID, buildID, keyboardID,
				[]string{deletedSwitch, resolvableSwitch})).To(Succeed())
			Expect(db.DeleteSwitch(ctx, ownerID, resolvableSwitch)).To(Succeed())
		})

		When("getting the build", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Get(ctx, ownerID, buildID, ownerToken)
				Expect(err).NotTo(HaveOccurred())
			})

			It("resolves the existing switch's entry without disturbing the deleted one's", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var got struct {
					Switches []struct {
						Switch *struct {
							ID    string `json:"id"`
							Brand string `json:"brand"`
							Name  string `json:"name"`
						} `json:"switch"`
						Count int `json:"count"`
					} `json:"switches"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Switches).To(HaveLen(2))

				Expect(got.Switches[0].Switch).To(BeNil())

				Expect(got.Switches[1].Switch).NotTo(BeNil())
				Expect(got.Switches[1].Switch.ID).To(Equal(resolvableSwitch))
				Expect(got.Switches[1].Switch.Brand).To(Equal("Gateron"))
				Expect(got.Switches[1].Switch.Name).To(Equal("Yellow"))
			})
		})
	})
})
