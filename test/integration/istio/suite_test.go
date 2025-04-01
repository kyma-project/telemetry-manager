//go:build istio

package istio

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestIstioIntegration(t *testing.T) {
	format.MaxDepth = suite.GomegaMaxDepth
	format.MaxLength = suite.GomegaMaxLenght

	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite - Istio")
}

var _ = BeforeSuite(suite.BeforeSuiteFunc)
var _ = AfterSuite(suite.AfterSuiteFunc)
