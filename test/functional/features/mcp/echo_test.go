package mcp_test

import (
	gomcp "github.com/mark3labs/mcp-go/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var _ = Describe("Echo", func() {
	var (
		client *support.MCPClient
		resp   *support.RPCResponse
	)

	BeforeEach(func() {
		client = nil
		resp = nil
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = support.NewMCPClient(support.BaseURL()+"/mcp", "")
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

		When("the echo tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				_, err := client.Initialize(ctx)
				Expect(err).NotTo(HaveOccurred())

				resp, err = client.CallTool(ctx, "echo", map[string]any{"message": "hello"})
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
			token, err := support.AuthToken(ctx)
			Expect(err).NotTo(HaveOccurred())
			client = support.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		When("the echo tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				_, err := client.Initialize(ctx)
				Expect(err).NotTo(HaveOccurred())

				resp, err = client.CallTool(ctx, "echo", map[string]any{"message": "hello from func test"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("echoes the message back", func() {
				var result gomcp.CallToolResult
				Expect(result.UnmarshalJSON(resp.Result)).To(Succeed())

				By("not returning an error result")
				Expect(result.IsError).To(BeFalse())

				By("returning exactly one content item")
				Expect(result.Content).To(HaveLen(1))

				By("echoing back the original message text")
				textContent, ok := result.Content[0].(gomcp.TextContent)
				Expect(ok).To(BeTrue())
				Expect(textContent.Text).To(Equal("hello from func test"))
			})
		})
	})
})
