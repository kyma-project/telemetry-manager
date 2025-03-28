//go:build e2e

package misc

import (
	"testing"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

func TestMisc(t *testing.T) {
	format.MaxDepth = suite.GomegaMaxDepth
	format.MaxLength = suite.GomegaMaxLenght

	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite - Misc")
}

var _ = BeforeSuite(suite.BeforeSuiteFunc)
var _ = AfterSuite(suite.AfterSuiteFunc)
