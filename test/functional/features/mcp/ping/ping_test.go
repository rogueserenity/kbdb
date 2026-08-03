package ping_test

import (
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

var _ = Describe("Ping", func() {
	var (
		client *api.MCPClient
		result *sdkmcp.CallToolResult
		err    error
	)

	BeforeEach(func() {
		client = nil
		result = nil
		err = nil
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the ping tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "ping", map[string]any{})
			})

			It("rejects the call with a real HTTP 401, per the spec's authorization error handling requirements", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(http.StatusText(http.StatusUnauthorized)))
			})
		})
	})

	Context("given a malformed bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "not-a-valid-jwt")
		})

		When("the ping tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "ping", map[string]any{})
			})

			It("rejects the call with a real HTTP 401, not silently falling back to anonymous", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(http.StatusText(http.StatusUnauthorized)))
			})
		})
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, err := api.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		When("the ping tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "ping", map[string]any{})
			})

			It("succeeds", func() {
				Expect(err).NotTo(HaveOccurred())

				By("not returning an error result")
				Expect(result.IsError).To(BeFalse())

				By("returning exactly one content item")
				Expect(result.Content).To(HaveLen(1))

				By("returning the trivial OK text")
				textContent, ok := result.Content[0].(*sdkmcp.TextContent)
				Expect(ok).To(BeTrue())
				Expect(textContent.Text).To(Equal("ok"))
			})
		})
	})
})
