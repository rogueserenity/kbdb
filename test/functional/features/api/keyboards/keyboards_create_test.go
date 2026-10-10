package keyboards_test

import (
	"encoding/json"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Creating a keyboard", func() {
	var (
		resp       *http.Response
		client     *api.KeyboardsClient
		ownerID    string
		ownerToken string
		createdID  string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil
		client = api.NewKeyboardsClient()
		createdID = ""

		var err error
		ownerToken, ownerID, err = api.NewAuthIdentity(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteKeyboard(ctx, ownerID, createdID)).To(Succeed())
		}
	})

	// captureCreatedID is called from BeforeEach, right after a successful
	// Create response is decoded - not from inside an It block after that
	// spec's own assertions. An assertion failing partway through an It
	// must not skip capturing this and leak the created keyboard into every
	// later spec that lists this owner's keyboards.
	captureCreatedID := func(r *http.Response) {
		var got struct {
			ID string `json:"id"`
		}
		Expect(json.NewDecoder(r.Body).Decode(&got)).To(Succeed())
		createdID = got.ID
	}

	Context("given the caller is the owner", func() {
		Context("given a valid request body", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the keyboard", func() {
					By("returning 201 Created")
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("returning a server-generated id")
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given every open-vocabulary field has an approved lookup value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private",`+
							`"size":"`+approvedSize+`","layout":"`+approvedLayout+`",`+
							`"design":{"top_case":{"material":"`+approvedCaseMaterial+`"}},`+
							`"purchase":{"vendor":"`+approvedVendor+`"}}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("creates the keyboard", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))
					Expect(createdID).NotTo(BeEmpty())
				})
			})
		})

		Context("given plates and PCBs, one plate with its own purchase", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","purchase":{"price":300},`+
							`"plates":[{"material":"AL","color":"Black","thickness":1.5},`+
							`{"material":"PC","purchase":{"vendor":"`+approvedVendor+`","price":35,"order_status":"Shipped"}}],`+
							`"pcbs":[{"firmware":"QMK/VIA","thickness":1.6}]}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("stores every part with a server-generated id and prices the extra plate", func(ctx SpecContext) {
					Expect(resp.StatusCode).To(Equal(http.StatusCreated))

					By("reading the keyboard back")
					getResp, err := client.Get(ctx, ownerID, createdID, ownerToken)
					Expect(err).NotTo(HaveOccurred())
					Expect(getResp.StatusCode).To(Equal(http.StatusOK))

					var got keyboardWithParts
					Expect(json.NewDecoder(getResp.Body).Decode(&got)).To(Succeed())

					By("giving each part its own id")
					Expect(got.Plates).To(HaveLen(2))
					Expect(got.PCBs).To(HaveLen(1))
					ids := []string{got.Plates[0].ID, got.Plates[1].ID, got.PCBs[0].ID}
					for _, id := range ids {
						Expect(id).NotTo(BeEmpty())
					}
					Expect(ids[0]).NotTo(Equal(ids[1]))

					By("keeping each part's fields, in order")
					Expect(got.Plates[0].Material).To(Equal("AL"))
					Expect(*got.Plates[0].Color).To(Equal("Black"))
					Expect(*got.Plates[0].Thickness).To(Equal(1.5))
					Expect(got.Plates[0].Purchase).To(BeNil())
					Expect(got.Plates[1].Material).To(Equal("PC"))
					Expect(got.Plates[1].Purchase).NotTo(BeNil())
					Expect(*got.Plates[1].Purchase.Vendor).To(Equal(approvedVendor))
					Expect(*got.Plates[1].Purchase.OrderStatus).To(Equal("Shipped"))
					Expect(*got.PCBs[0].Firmware).To(Equal("QMK/VIA"))

					By("adding the plate's price to the base price in total_cost")
					Expect(got.TotalCost).NotTo(BeNil())
					Expect(*got.TotalCost).To(Equal(335.0))
				})
			})
		})

		Context("given a part with an id", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","plates":[{"id":"made-up","material":"AL"}]}`)
					Expect(err).NotTo(HaveOccurred())
					if resp.StatusCode == http.StatusCreated {
						captureCreatedID(resp)
					}
				})

				It("returns 400 naming the part's id", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

					var got struct {
						InvalidParams []struct {
							Name string `json:"name"`
						} `json:"invalid_params"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					Expect(got.InvalidParams).To(ConsistOf(HaveField("Name", "plates[0].id")))
				})
			})
		})

		Context("given an open-vocabulary field has an unapproved value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","size":"NotApproved"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given size is approved but not valid for the given layout", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","size":"`+approvedOtherSize+`","layout":"`+approvedLayout+`"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body naming only layout", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))

					var got struct {
						InvalidParams []struct {
							Name string `json:"name"`
						} `json:"invalid_params"`
					}
					Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
					names := make([]string, len(got.InvalidParams))
					for i, p := range got.InvalidParams {
						names[i] = p.Name
					}
					Expect(names).To(ConsistOf("layout"))
				})
			})
		})

		Context("given layout is not a recognized value", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken,
						`{"brand":"Keychron","name":"Q1","visibility":"private","layout":"NotApproved"}`)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns 400 with a problem+json body", func() {
					Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
					Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
				})
			})
		})

		Context("given required fields are missing", func() {
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, ownerToken, `{"brand":"Keychron"}`)
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
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "",
						`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
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
			When("creating a keyboard", func() {
				BeforeEach(func(ctx SpecContext) {
					var err error
					resp, err = client.Create(ctx, ownerID, "", `{"brand":"Keychron"}`)
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
			token, _, err = api.NewAuthIdentity(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("creating a keyboard in the owner's collection", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Create(ctx, ownerID, token,
					`{"brand":"Keychron","name":"Q1","visibility":"private"}`)
				Expect(err).NotTo(HaveOccurred())
			})

			It("returns 404, not 403, to avoid revealing whose collection this is", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})
})
