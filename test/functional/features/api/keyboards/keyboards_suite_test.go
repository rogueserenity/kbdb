package keyboards_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKeyboards(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keyboards Suite")
}
