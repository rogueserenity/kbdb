package authorize_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAuthorize(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GET /authorize Suite")
}
