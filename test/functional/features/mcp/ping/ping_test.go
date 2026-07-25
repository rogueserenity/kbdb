package ping_test

import (
	gomcp "github.com/mark3labs/mcp-go/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

var _ = Describe("Ping", func() {
	var (
		client *api.MCPClient
		resp   *api.RPCResponse
	)

	BeforeEach(func() {
		client = nil
		resp = nil
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the initialize handshake is performed", func() {
			BeforeEach(func(ctx SpecContext) {
				var err error
				resp, err = client.Initialize(ctx)
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				By("returning no protocol-level error")
				Expect(resp.Error).To(BeEmpty())
			})
		})

		When("the ping tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				_, err := client.Initialize(ctx)
				Expect(err).NotTo(HaveOccurred())

				resp, err = client.CallTool(ctx, "ping", map[string]any{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("rejects the call", func() {
				var result gomcp.CallToolResult
				Expect(result.UnmarshalJSON(resp.Result)).To(Succeed())

				By("returning an MCP-shaped error result, not a bare HTTP failure")
				Expect(result.IsError).To(BeTrue())
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
				_, err := client.Initialize(ctx)
				Expect(err).NotTo(HaveOccurred())

				resp, err = client.CallTool(ctx, "ping", map[string]any{})
				Expect(err).NotTo(HaveOccurred())
			})

			It("succeeds", func() {
				var result gomcp.CallToolResult
				Expect(result.UnmarshalJSON(resp.Result)).To(Succeed())

				By("not returning an error result")
				Expect(result.IsError).To(BeFalse())

				By("returning exactly one content item")
				Expect(result.Content).To(HaveLen(1))

				By("returning the trivial OK text")
				textContent, ok := result.Content[0].(gomcp.TextContent)
				Expect(ok).To(BeTrue())
				Expect(textContent.Text).To(Equal("ok"))
			})
		})
	})
})
