package log

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCustomMatchers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log Matcher Suite")
}
