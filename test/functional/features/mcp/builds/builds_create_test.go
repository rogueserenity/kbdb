package builds_test

import (
	"github.com/google/uuid"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rogueserenity/kbdb/test/functional/support"
	"github.com/rogueserenity/kbdb/test/functional/support/api"
	"github.com/rogueserenity/kbdb/test/functional/support/db"
)

var _ = Describe("Creating a build over MCP", func() {
	var (
		client     *api.MCPClient
		result     *sdkmcp.CallToolResult
		err        error
		ownerID    string
		keyboardID string
		createdID  string
	)

	BeforeEach(func() {
		result = nil
		err = nil
		createdID = ""
	})

	captureCreatedID := func(r *sdkmcp.CallToolResult) {
		if r == nil || r.IsError {
			return
		}
		createdID = decodeBuildOutput(r).Build.ID
	}

	AfterEach(func(ctx SpecContext) {
		if createdID != "" {
			Expect(db.DeleteBuild(ctx, ownerID, createdID)).To(Succeed())
		}
		if keyboardID != "" {
			Expect(db.DeleteKeyboard(ctx, ownerID, keyboardID)).To(Succeed())
		}
	})

	Context("given a valid bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())

			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			client = api.NewMCPClient(support.BaseURL()+"/mcp", token)

			// A build references a keyboard, so seed one directly rather
			// than through the API - keeps this spec focused on the
			// create_build tool itself.
			keyboardID = "build-fixture-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		})

		Context("given valid arguments", func() {
			When("the create_build tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_build", map[string]any{
						"keyboard":   keyboardID,
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("creates the build in the caller's own collection", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())

					out := decodeBuildOutput(result)
					Expect(out.Build.ID).NotTo(BeEmpty(), "the server assigns the id")
					Expect(out.Build.Keyboard).To(Equal(keyboardID))
					Expect(out.Build.Visibility).To(Equal("private"))
					Expect(out.Build.HasImages).To(BeFalse())
				})
			})
		})

		Context("given every open-vocabulary field has an approved lookup value", func() {
			When("the create_build tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_build", map[string]any{
						"keyboard":   keyboardID,
						"visibility": "private",
						"stabs": map[string]any{
							"name":       approvedStabilizer,
							"mount_type": approvedStabilizerMount,
						},
						"case_mount_type": map[string]any{
							"type":      approvedCaseMountType,
							"durometer": approvedDurometer,
						},
					})
					captureCreatedID(result)
				})

				It("creates the build", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeFalse())
					Expect(decodeBuildOutput(result).Build.ID).NotTo(BeEmpty())
				})
			})
		})

		Context("given an open-vocabulary field has an unapproved value", func() {
			When("the create_build tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_build", map[string]any{
						"keyboard":   keyboardID,
						"visibility": "private",
						"stabs":      map[string]any{"name": "NotApproved"},
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given a required field is blank", func() {
			When("the create_build tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_build", map[string]any{
						"keyboard":   "",
						"visibility": "private",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})

		Context("given an invalid visibility", func() {
			When("the create_build tool is called", func() {
				BeforeEach(func(ctx SpecContext) {
					result, err = client.CallTool(ctx, "create_build", map[string]any{
						"keyboard":   keyboardID,
						"visibility": "everyone",
					})
					captureCreatedID(result)
				})

				It("returns an MCP tool error result", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(result.IsError).To(BeTrue())
				})
			})
		})
	})

	Context("given no bearer token", func() {
		BeforeEach(func(ctx SpecContext) {
			client = api.NewMCPClient(support.BaseURL()+"/mcp", "")

			token, tokenErr := api.AuthToken(ctx)
			Expect(tokenErr).NotTo(HaveOccurred())
			ownerID, err = api.TokenSubject(token)
			Expect(err).NotTo(HaveOccurred())

			keyboardID = "build-fixture-keyboard-" + uuid.NewString()
			Expect(db.SeedKeyboard(ctx, ownerID, keyboardID, "private")).To(Succeed())
		})

		When("the create_build tool is called", func() {
			BeforeEach(func(ctx SpecContext) {
				result, err = client.CallTool(ctx, "create_build", map[string]any{
					"keyboard":   keyboardID,
					"visibility": "private",
				})
			})

			It("rejects the call with a real HTTP 401", func() {
				Expect(result).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
