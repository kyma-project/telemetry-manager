package misc

import (
	"testing"

	. "github.com/onsi/gomega"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestFilterInvalid(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsMisc, suite.LabelExperimental)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
	)

	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithFilter(telemetryv1beta1.FilterSpec{
			Conditions: []string{
				`Len(resource.attributes["k8s.namespace.name"]) > 0`, // perfectly valid condition with context prefix
				`attributes["foo"] == "bar"`,                         // invalid condition (missing context prefix)
			},
		}).
		WithOTLPOutput(testutils.OTLPEndpoint("https://backend.example.com:4317")).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline)).ToNot(Succeed())
}
