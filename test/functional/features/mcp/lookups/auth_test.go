package lookups_test

import (
	"net/http"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
)

// list_lookups is the vehicle here only because it needs no fixtures - the
// property under test is the transport's auth, which is shared by every
// tool.
var _ = Describe("Calling an MCP tool with a bad token", func() {
	var (
		client *api.MCPClient
		result *sdkmcp.CallToolResult
		err    error
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a malformed bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "not-a-valid-jwt")
		})

		When("a tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_lookups", map[string]any{})
			})

			It("rejects the call with a real HTTP 401, not silently falling back to anonymous", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(http.StatusText(http.StatusUnauthorized)))
			})
		})
	})
})
