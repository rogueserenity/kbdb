package api_test

import (
	"bytes"
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

var _ = Describe("Lookups", func() {
	var (
		resp     *http.Response
		category string
	)

	BeforeEach(func() {
		resp = nil
		category = "functional-test-category"
	})

	AfterEach(func(ctx SpecContext) {
		if resp != nil {
			_ = resp.Body.Close()
		}

		client := lookupDynamoClient(ctx)
		_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(support.LookupTableName()),
			Key: map[string]dynamotypes.AttributeValue{
				"category": &dynamotypes.AttributeValueMemberS{Value: category},
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("given the lookup table has a category", func() {
		BeforeEach(func(ctx SpecContext) {
			seedCategory(ctx, category, []string{"a", "b"})
		})

		When("the request is made with no bearer token", func() {
			BeforeEach(func(ctx SpecContext) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/lookups", nil)
				Expect(err).NotTo(HaveOccurred())

				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds and includes the seeded category", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("including the seeded category name in the response body")
				var categories []string
				Expect(json.NewDecoder(resp.Body).Decode(&categories)).To(Succeed())
				Expect(categories).To(ContainElement(category))
			})
		})

		When("the request is for the seeded category, with no bearer token", func() {
			BeforeEach(func(ctx SpecContext) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/lookups/"+category, nil)
				Expect(err).NotTo(HaveOccurred())

				resp, err = http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds and returns the category's values", func() {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("returning the category's name and values")
				var got struct {
					Category string   `json:"category"`
					Values   []string `json:"values"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Category).To(Equal(category))
				Expect(got.Values).To(Equal([]string{"a", "b"}))
			})
		})
	})

	When("the request is for a category that does not exist", func() {
		BeforeEach(func(ctx SpecContext) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/lookups/functional-test-category-missing", nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err = http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 404 with a problem+json body", func() {
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
		})
	})

	Describe("POST /v1/lookups/{category}", func() {
		doCreateWithBody := func(ctx SpecContext, token, requestBody string) *http.Response {
			body := bytes.NewReader([]byte(requestBody))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, support.BaseURL()+"/v1/lookups/"+category, body)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			r, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return r
		}
		doCreate := func(ctx SpecContext, token string) *http.Response {
			return doCreateWithBody(ctx, token, `{"values":["a","b"]}`)
		}

		When("the caller is an admin and the category does not exist", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doCreate(ctx, token)
			})

			It("creates the category", func() {
				By("returning 201 Created")
				Expect(resp.StatusCode).To(Equal(http.StatusCreated))

				By("returning the created category's name and values")
				var got struct {
					Category string   `json:"category"`
					Values   []string `json:"values"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Category).To(Equal(category))
				Expect(got.Values).To(Equal([]string{"a", "b"}))
			})
		})

		When("the caller is an admin and the category already exists", func() {
			BeforeEach(func(ctx SpecContext) {
				seedCategory(ctx, category, []string{"a", "b"})

				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doCreate(ctx, token)
			})

			It("returns 409 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusConflict))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("the caller is not an admin", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doCreate(ctx, token)
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("no bearer token is provided", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doCreate(ctx, "")
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})

		When("the caller is an admin but values is empty", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doCreateWithBody(ctx, token, `{"values":[]}`)
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})
	})

	Describe("PUT /v1/lookups/{category}", func() {
		doReplaceWithBody := func(ctx SpecContext, token, requestBody string) *http.Response {
			body := bytes.NewReader([]byte(requestBody))
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, support.BaseURL()+"/v1/lookups/"+category, body)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "application/json")
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			r, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return r
		}
		doReplace := func(ctx SpecContext, token string) *http.Response {
			return doReplaceWithBody(ctx, token, `{"values":["c","d"]}`)
		}

		When("the caller is an admin and the category exists", func() {
			BeforeEach(func(ctx SpecContext) {
				seedCategory(ctx, category, []string{"a", "b"})

				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doReplace(ctx, token)
			})

			It("replaces the category's values", func(ctx SpecContext) {
				By("returning 200 OK")
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				By("returning the new values in the response body")
				var got struct {
					Category string   `json:"category"`
					Values   []string `json:"values"`
				}
				Expect(json.NewDecoder(resp.Body).Decode(&got)).To(Succeed())
				Expect(got.Category).To(Equal(category))
				Expect(got.Values).To(Equal([]string{"c", "d"}))

				By("actually persisting the new values, not a no-op")
				getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/lookups/"+category, nil)
				Expect(err).NotTo(HaveOccurred())
				getResp, err := http.DefaultClient.Do(getReq)
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = getResp.Body.Close() }()
				Expect(getResp.StatusCode).To(Equal(http.StatusOK))

				var reGot struct {
					Values []string `json:"values"`
				}
				Expect(json.NewDecoder(getResp.Body).Decode(&reGot)).To(Succeed())
				Expect(reGot.Values).To(Equal([]string{"c", "d"}))
			})
		})

		When("the caller is an admin and the category does not exist", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doReplace(ctx, token)
			})

			It("returns 404 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("the caller is an admin but values is empty", func() {
			BeforeEach(func(ctx SpecContext) {
				seedCategory(ctx, category, []string{"a", "b"})

				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doReplaceWithBody(ctx, token, `{"values":[]}`)
			})

			It("returns 400 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("the caller is not an admin", func() {
			BeforeEach(func(ctx SpecContext) {
				seedCategory(ctx, category, []string{"a", "b"})

				token, err := support.AuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doReplace(ctx, token)
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("no bearer token is provided", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doReplace(ctx, "")
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("DELETE /v1/lookups/{category}", func() {
		doDelete := func(ctx SpecContext, token string) *http.Response {
			req, err := http.NewRequestWithContext(ctx, http.MethodDelete, support.BaseURL()+"/v1/lookups/"+category, nil)
			Expect(err).NotTo(HaveOccurred())
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			r, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return r
		}

		When("the caller is an admin and the category exists", func() {
			BeforeEach(func(ctx SpecContext) {
				seedCategory(ctx, category, []string{"a", "b"})

				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doDelete(ctx, token)
			})

			It("deletes the category", func(ctx SpecContext) {
				By("returning 204 No Content")
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))

				By("actually removing the category, not a no-op")
				getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, support.BaseURL()+"/v1/lookups/"+category, nil)
				Expect(err).NotTo(HaveOccurred())
				getResp, err := http.DefaultClient.Do(getReq)
				Expect(err).NotTo(HaveOccurred())
				defer func() { _ = getResp.Body.Close() }()
				Expect(getResp.StatusCode).To(Equal(http.StatusNotFound))
			})
		})

		When("the caller is an admin and the category never existed", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AdminAuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doDelete(ctx, token)
			})

			It("returns 204 (idempotent, not 404)", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusNoContent))
			})
		})

		When("the caller is not an admin", func() {
			BeforeEach(func(ctx SpecContext) {
				token, err := support.AuthToken(ctx)
				Expect(err).NotTo(HaveOccurred())
				resp = doDelete(ctx, token)
			})

			It("returns 403 with a problem+json body", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
				Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"))
			})
		})

		When("no bearer token is provided", func() {
			BeforeEach(func(ctx SpecContext) {
				resp = doDelete(ctx, "")
			})

			It("returns 401", func() {
				Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})

func lookupDynamoClient(ctx context.Context) *dynamodb.Client {
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

// seedCategory PutItems a lookup category directly into DynamoDB, bypassing
// the API - used to set up state for specs that exercise a different route.
func seedCategory(ctx SpecContext, category string, values []string) {
	client := lookupDynamoClient(ctx)
	item, err := attributevalue.MarshalMap(map[string]any{
		"category": category,
		"values":   values,
	})
	Expect(err).NotTo(HaveOccurred())

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(support.LookupTableName()),
		Item:      item,
	})
	Expect(err).NotTo(HaveOccurred())
}
