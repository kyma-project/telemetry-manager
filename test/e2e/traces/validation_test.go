package traces

import (
	"testing"

	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestCELRules(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName("misconfigured-secretref-pipeline").
		WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "")).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline)).ShouldNot(Succeed())
}
