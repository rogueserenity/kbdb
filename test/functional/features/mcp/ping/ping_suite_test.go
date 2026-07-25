package ping_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPing(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ping Suite")
}
