package switches_test

import (
	"encoding/json"

	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

type listOutput struct {
	Switches []struct {
		ID    string `json:"id"`
		Brand string `json:"brand"`
		Name  string `json:"name"`
		Type  string `json:"type"`
	} `json:"switches"`
	NextCursor string `json:"next_cursor"`
}

func decodeListOutput(result *sdkmcp.CallToolResult) listOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out listOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

func idsOf(out listOutput) []string {
	ids := make([]string, 0, len(out.Switches))
	for _, sw := range out.Switches {
		ids = append(ids, sw.ID)
	}

	return ids
}

var _ = Describe("Listing switches over MCP", func() {
	var (
		client  *api.MCPClient
		result  *sdkmcp.CallToolResult
		err     error
		ownerID string
	)

	BeforeEach(func() {
		result = nil
		err = nil
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			var token string
			token, ownerID, err = api.NewAuthIdentity(ctx)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)
		})

		Context("given the caller owns a private switch", func() {
			var switchID string

			BeforeEach(func(ctx SpecContext) {
				switchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
			})

			When("the list_switches tool is called with no user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{})
				})

				It("defaults to the caller's own collection and includes the private switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).To(ContainElement(switchID))
				})
			})

			DescribeTable("given an out-of-range limit",
				func(ctx SpecContext, limit int) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"limit": limit})
					Expect(err).NotTo(HaveOccurred())

					By("clamping rather than rejecting the call")
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).To(ContainElement(switchID))
				},
				Entry("below the minimum", 0),
				Entry("above the maximum", 101),
			)
		})

		Context("given another user owns a private switch", func() {
			var (
				otherID       string
				otherSwitchID string
			)

			BeforeEach(func(ctx SpecContext) {
				_, otherID, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())

				otherSwitchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, otherID, otherSwitchID, "private")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, otherSwitchID)).To(Succeed())
			})

			When("the list_switches tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"user_id": otherID})
				})

				It("omits the other user's private switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).NotTo(ContainElement(otherSwitchID))
				})
			})
		})

		Context("given another user owns a public switch", func() {
			var (
				otherID       string
				otherSwitchID string
			)

			BeforeEach(func(ctx SpecContext) {
				_, otherID, err = api.NewAuthIdentity(ctx)
				Expect(err).NotTo(HaveOccurred())

				otherSwitchID = "functional-test-switch-" + uuid.NewString()
				Expect(db.SeedSwitch(ctx, otherID, otherSwitchID, "public")).To(Succeed())
			})

			AfterEach(func(ctx SpecContext) {
				Expect(db.DeleteSwitch(ctx, otherID, otherSwitchID)).To(Succeed())
			})

			When("the list_switches tool is called with that user_id", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "list_switches", map[string]any{"user_id": otherID})
				})

				It("includes the other user's public switch", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(idsOf(decodeListOutput(result))).To(ContainElement(otherSwitchID))
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func() {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")
		})

		When("the list_switches tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "list_switches", map[string]any{})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
