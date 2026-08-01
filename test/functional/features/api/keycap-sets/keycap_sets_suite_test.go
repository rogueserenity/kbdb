package keycapsets_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestKeycapSets(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keycap Sets Suite")
}
