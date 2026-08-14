package builds_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBuilds(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Builds Suite")
}

// approvedStabilizer/approvedStabilizerMountType/approvedCaseMountType/
// approvedDurometer are real values from internal/lookup/data/, matching
// what the REST builds suite uses for the same lookup categories.
const (
	approvedStabilizer      = "Durock v3"
	approvedStabilizerMount = "Screw-in"
	approvedCaseMountType   = "Gasket Mount"
	approvedDurometer       = "70A"
)

type build struct {
	ID         string `json:"id"`
	Keyboard   string `json:"keyboard"`
	Visibility string `json:"visibility"`
	HasImages  bool   `json:"has_images"`
}

type createOutput struct {
	Build build `json:"build"`
}

func decodeBuildOutput(result *sdkmcp.CallToolResult) createOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out createOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

type listedBuildKeyboard struct {
	Brand string `json:"brand"`
	Name  string `json:"name"`
}

type listedBuild struct {
	ID       string               `json:"id"`
	Keyboard *listedBuildKeyboard `json:"keyboard"`
}

type listBuildsOutput struct {
	Builds     []listedBuild `json:"builds"`
	NextCursor string        `json:"next_cursor"`
}

func decodeListBuildsOutput(result *sdkmcp.CallToolResult) listBuildsOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out listBuildsOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

func buildIDsOf(out listBuildsOutput) []string {
	ids := make([]string, 0, len(out.Builds))
	for _, b := range out.Builds {
		ids = append(ids, b.ID)
	}

	return ids
}
