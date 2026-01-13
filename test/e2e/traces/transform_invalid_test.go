package traces

import (
	"testing"

	. "github.com/onsi/gomega"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestTransformInvalid(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithTransform(telemetryv1beta1.TransformSpec{
			Statements: []string{"sset(span.attributes[\"test\"], \"foo\")"},
		}).
		WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com:4317")).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline)).ToNot(Succeed())
}
