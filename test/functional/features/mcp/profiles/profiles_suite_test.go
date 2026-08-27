package profiles_test

import (
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProfiles(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Profiles Suite")
}

type getProfileOutput struct {
	Profile struct {
		Username        string  `json:"username"`
		Discoverable    bool    `json:"discoverable"`
		DiscordUsername *string `json:"discord_username"`
		Bio             *string `json:"bio"`
		Links           []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"links"`
		HasAvatar bool `json:"has_avatar"`
	} `json:"profile"`
}

func decodeGetProfileOutput(result *sdkmcp.CallToolResult) getProfileOutput {
	GinkgoHelper()

	raw, err := json.Marshal(result.StructuredContent)
	Expect(err).NotTo(HaveOccurred())

	var out getProfileOutput
	Expect(json.Unmarshal(raw, &out)).To(Succeed())

	return out
}
