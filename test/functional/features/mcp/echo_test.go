package mcp_test

import (
	gomcp "github.com/mark3labs/mcp-go/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
)

var _ = Describe("MCP echo tool", func() {
	It("allows the initialize handshake without auth", func() {
		client := support.NewMCPClient(support.BaseURL()+"/mcp", "")

		resp, err := client.Initialize()
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Error).To(BeEmpty())
	})

	It("returns an MCP-shaped error calling echo without a token", func() {
		client := support.NewMCPClient(support.BaseURL()+"/mcp", "")

		_, err := client.Initialize()
		Expect(err).NotTo(HaveOccurred())

		resp, err := client.CallTool("echo", map[string]any{"message": "hello"})
		Expect(err).NotTo(HaveOccurred())

		var result gomcp.CallToolResult
		Expect(result.UnmarshalJSON(resp.Result)).To(Succeed())
		Expect(result.IsError).To(BeTrue())
	})

	It("echoes the message back with a valid token", func() {
		token, err := support.AuthToken()
		Expect(err).NotTo(HaveOccurred())

		client := support.NewMCPClient(support.BaseURL()+"/mcp", token)

		_, err = client.Initialize()
		Expect(err).NotTo(HaveOccurred())

		resp, err := client.CallTool("echo", map[string]any{"message": "hello from func test"})
		Expect(err).NotTo(HaveOccurred())

		var result gomcp.CallToolResult
		Expect(result.UnmarshalJSON(resp.Result)).To(Succeed())
		Expect(result.IsError).To(BeFalse())
		Expect(result.Content).To(HaveLen(1))

		textContent, ok := result.Content[0].(gomcp.TextContent)
		Expect(ok).To(BeTrue())
		Expect(textContent.Text).To(Equal("hello from func test"))
	})
})
