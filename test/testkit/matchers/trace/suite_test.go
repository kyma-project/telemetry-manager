package trace

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTraceMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Trace Matcher Suite")
}
