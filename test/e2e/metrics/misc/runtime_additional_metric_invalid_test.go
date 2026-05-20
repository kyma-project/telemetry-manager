package misc

import (
	"testing"

	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestRuntimeAdditionalMetricInvalid(t *testing.T) {
	suite.SetupTest(t, suite.LabelMetrics, suite.LabelMisc)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true).
		WithRuntimeInputAdditionalMetrics("k8s.pod.invalid_metric").
		WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com:4317")).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline)).ToNot(Succeed())
}
