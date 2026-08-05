package keyboards_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Keyboards Suite")
}

type getOutput struct {
	Keyboard struct {
		ID         string  `json:"id"`
		Brand      string  `json:"brand"`
		Name       string  `json:"name"`
		Size       *string `json:"size"`
		Visibility string  `json:"visibility"`
	} `json:"keyboard"`
}

func decodeGetOutput(result *sdkmcp.CallToolResult) getOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out getOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}

type listOutput struct {
	Keyboards []struct {
		ID          string  `json:"id"`
		Brand       string  `json:"brand"`
		Name        string  `json:"name"`
		OrderStatus *string `json:"order_status"`
	} `json:"keyboards"`
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
	ids := make([]string, 0, len(out.Keyboards))
	for _, kb := range out.Keyboards {
		ids = append(ids, kb.ID)
	}

	return ids
}
