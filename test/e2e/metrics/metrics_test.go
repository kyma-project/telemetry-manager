//go:build e2e

package metrics

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestMisc(t *testing.T) {
	format.MaxDepth = GomegaMaxDepth
	format.MaxLength = GomegaMaxLenght

	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite - Metrics")
}

var _ = BeforeSuite(BeforeSuiteFunc)
var _ = AfterSuite(AfterSuiteFunc)
