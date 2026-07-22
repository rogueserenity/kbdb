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
			client := lookupDynamoClient(ctx)
			item, err := attributevalue.MarshalMap(map[string]any{
				"category": category,
				"values":   []string{"a", "b"},
			})
			Expect(err).NotTo(HaveOccurred())

			_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
				TableName: aws.String(support.LookupTableName()),
				Item:      item,
			})
			Expect(err).NotTo(HaveOccurred())
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
