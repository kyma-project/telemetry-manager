//go:build e2e

package metrics

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"

	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestSuite(t *testing.T) {
	format.MaxDepth = suite.GomegaMaxDepth
	format.MaxLength = suite.GomegaMaxLenght

	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite - Metrics")
}

var _ = BeforeSuite(suite.BeforeSuiteFunc())
var _ = AfterSuite(suite.AfterSuiteFunc())
