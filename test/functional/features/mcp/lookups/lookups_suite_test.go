package lookups_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLookups(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MCP Lookups Suite")
}
