package switches_test

import (
	"encoding/json"

	"github.com/google/uuid"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Deleting a switch that is still referenced by a build, over MCP", func() {
	var (
		client     *api.MCPClient
		ownerID    string
		switchID   string
		buildID    string
		switchGone bool
		buildGone  bool
	)

	BeforeEach(func(ctx SpecContext) {
		switchGone = false
		buildGone = false
		switchID = "mcp-cascade-switch-" + uuid.NewString()
		buildID = "mcp-cascade-build-" + uuid.NewString()

		token, tokenErr := api.AuthToken(ctx)
		Expect(tokenErr).NotTo(HaveOccurred())

		var err error
		ownerID, err = api.TokenSubject(token)
		Expect(err).NotTo(HaveOccurred())

		client = api.NewMCPClient(support.BaseURL()+"/mcp", token)

		Expect(db.SeedSwitch(ctx, ownerID, switchID, "private")).To(Succeed())
		Expect(db.SeedBuildWithSwitch(ctx, ownerID, buildID, switchID, "private")).To(Succeed())
	})

	AfterEach(func(ctx SpecContext) {
		if !buildGone {
			Expect(db.DeleteBuildWithSwitch(ctx, ownerID, buildID, switchID)).To(Succeed())
		}
		if !switchGone {
			Expect(db.DeleteSwitch(ctx, ownerID, switchID)).To(Succeed())
		}
	})

	Context("given on_delete is omitted (defaults to block)", func() {
		When("the delete_switch tool is called", func() {
			It("fails and lists the blocking build id, leaving both in place", func(ctx SpecContext) {
				result, err := client.CallTool(ctx, "delete_switch", map[string]any{
					"switch_id": switchID,
				})
				Expect(err).NotTo(HaveOccurred())

				By("returning a tool error")
				Expect(result.IsError).To(BeTrue())

				By("the switch still existing")
				getSw, getErr := client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getSw.IsError).To(BeFalse())

				By("the build still existing")
				getBuild, getErr := client.CallTool(ctx, "get_build", map[string]any{"build_id": buildID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getBuild.IsError).To(BeFalse())
			})
		})
	})

	Context("given on_delete is cascade", func() {
		When("the delete_switch tool is called", func() {
			It("deletes the switch and the referencing build, returning the deleted build id", func(ctx SpecContext) {
				result, err := client.CallTool(ctx, "delete_switch", map[string]any{
					"switch_id": switchID,
					"on_delete": "cascade",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
				switchGone = true
				buildGone = true

				By("the tool output listing the deleted build id")
				raw, marshalErr := json.Marshal(result.StructuredContent)
				Expect(marshalErr).NotTo(HaveOccurred())

				var out struct {
					DeletedBuildIDs []string `json:"deleted_build_ids"`
				}
				Expect(json.Unmarshal(raw, &out)).To(Succeed())
				Expect(out.DeletedBuildIDs).To(ContainElement(buildID))

				By("the switch no longer existing")
				getSw, getErr := client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getSw.IsError).To(BeTrue())

				By("the build no longer existing")
				getBuild, getErr := client.CallTool(ctx, "get_build", map[string]any{"build_id": buildID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getBuild.IsError).To(BeTrue())
			})
		})
	})

	Context("given on_delete is detach", func() {
		When("the delete_switch tool is called", func() {
			It("deletes the switch but leaves the build with a dangling switch reference", func(ctx SpecContext) {
				result, err := client.CallTool(ctx, "delete_switch", map[string]any{
					"switch_id": switchID,
					"on_delete": "detach",
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
				switchGone = true

				By("the switch no longer existing")
				getSw, getErr := client.CallTool(ctx, "get_switch", map[string]any{"switch_id": switchID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getSw.IsError).To(BeTrue())

				By("the build still existing, still referencing the deleted switch id")
				getBuild, getErr := client.CallTool(ctx, "get_build", map[string]any{"build_id": buildID})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getBuild.IsError).To(BeFalse())

				raw, marshalErr := json.Marshal(getBuild.StructuredContent)
				Expect(marshalErr).NotTo(HaveOccurred())

				var buildOut struct {
					Build struct {
						Switches []struct {
							Switch string `json:"switch"`
						} `json:"switches"`
					} `json:"build"`
				}
				Expect(json.Unmarshal(raw, &buildOut)).To(Succeed())
				Expect(buildOut.Build.Switches).To(HaveLen(1))
				Expect(buildOut.Build.Switches[0].Switch).To(Equal(switchID))
			})
		})
	})
})
