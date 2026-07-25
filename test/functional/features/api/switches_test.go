package api_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var _ = Describe("List switches", func() {
	var (
		resp       *http.Response
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil

		// Derived from a freshly minted token, not a fixed fixture subject:
		// in CI, AuthToken mints a real Cognito-generated subject rather
		// than mockoidc's fixtures.TestUserSubject string, so the owner
		// used to seed fixture data below must match whatever subject this
		// environment's token actually carries.
		var err error
		ownerToken, err = support.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = support.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if resp != nil {
			_ = resp.Body.Close()
		}

		client := switchDynamoClient(ctx)
		for _, id := range []string{"public-switch", "authenticated-switch", "private-switch"} {
			_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
				TableName: aws.String(support.SwitchTableName()),
				Key: map[string]dynamotypes.AttributeValue{
					"user_id": &dynamotypes.AttributeValueMemberS{Value: ownerID},
					"id":      &dynamotypes.AttributeValueMemberS{Value: id},
				},
			})
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("given the owner has switches at every visibility tier", func() {
		BeforeEach(func(ctx SpecContext) {
			seedSwitch(ctx, ownerID, "public-switch", "public")
			seedSwitch(ctx, ownerID, "authenticated-switch", "authenticated")
			seedSwitch(ctx, ownerID, "private-switch", "private")
		})

		doList := func(ctx SpecContext, token string) *http.Response {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				support.BaseURL()+"/users/"+ownerID+"/switches", nil)
			Expect(err).NotTo(HaveOccurred())
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			r, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return r
		}

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

		When("the request is made by the owner", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doList(ctx, ownerToken)
			})

			It("returns switches at every visibility tier", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("including all three seeded switches")
				Expect(itemIDs(resp)).To(ConsistOf("public-switch", "authenticated-switch", "private-switch"))
			})
		})

		When("the request is made with no bearer token", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doList(ctx, "")
			})

			It("returns only the public switch", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(itemIDs(resp)).To(ConsistOf("public-switch"))
			})
		})

		When("the request is made by a different authenticated user", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doList(ctx, token)
			})

			It("returns the public and authenticated switches, but not the private one", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusOK))
				Expect(itemIDs(resp)).To(ConsistOf("public-switch", "authenticated-switch"))
			})
		})
	})
})

var _ = Describe("Get switch", func() {
	var (
		resp       *http.Response
		ownerID    string
		ownerToken string
	)

	BeforeEach(func(ctx SpecContext) {
		resp = nil

		var err error
		ownerToken, err = support.AuthToken(ctx)
		Expect(err).NotTo(HaveOccurred())
		ownerID, err = support.TokenSubject(ownerToken)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func(ctx SpecContext) {
		if resp != nil {
			_ = resp.Body.Close()
		}

		client := switchDynamoClient(ctx)
		_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(support.SwitchTableName()),
			Key: map[string]dynamotypes.AttributeValue{
				"user_id": &dynamotypes.AttributeValueMemberS{Value: ownerID},
				"id":      &dynamotypes.AttributeValueMemberS{Value: "get-switch"},
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	doGet := func(ctx SpecContext, token string) *http.Response {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet,
			support.BaseURL()+"/users/"+ownerID+"/switches/get-switch", nil)
		Expect(err).NotTo(HaveOccurred())
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		r, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		return r
	}

	Context("given a private switch owned by the caller", func() {
		BeforeEach(func(ctx SpecContext) {
			seedSwitch(ctx, ownerID, "get-switch", "private")
		})

		When("the request is made by the owner", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doGet(ctx, ownerToken)
			})

			It("returns the switch", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("returning the switch's id")
				var got struct {
					ID string `json:"id"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.ID).To(Equal("get-switch"))
			})
		})

		When("the request is made by a different authenticated user", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.SecondUserAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doGet(ctx, token)
			})

			It("returns 404, not 403, to avoid revealing the item exists", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("the request is made with no bearer token", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doGet(ctx, "")
			})

			It("returns 404", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	When("the switch does not exist", func() {
		BeforeEach(func(ctx SpecContext) {
			resp = doGet(ctx, ownerToken)
		})

		It("returns 404", func() {
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
		})
	})
})

func switchDynamoClient(ctx context.Context) *dynamodb.Client {
	endpoint := support.DynamoDBEndpointURL()

	opts := []func(*awsconfig.LoadOptions) error{}
	if endpoint != "" {
		opts = append(opts, awsconfig.WithRegion("us-east-2"))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	Expect(err).NotTo(HaveOccurred())

	if endpoint != "" {
		awsCfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
	}

	return dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
}

// seedSwitch PutItems a switch directly into DynamoDB, bypassing the API -
// used to set up state for specs that exercise a different route.
func seedSwitch(ctx SpecContext, ownerID, id, visibility string) {
	client := switchDynamoClient(ctx)
	item, err := attributevalue.MarshalMap(map[string]any{
		"user_id":    ownerID,
		"id":         id,
		"brand":      "Gateron",
		"name":       "Yellow",
		"type":       "Linear",
		"visibility": visibility,
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(support.SwitchTableName()),
		Item:      item,
	})
	Expect(err).NotTo(HaveOccurred())
}
