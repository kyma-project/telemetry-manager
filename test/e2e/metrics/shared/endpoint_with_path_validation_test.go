package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestEndpointWithPathValidation(t *testing.T) {
	tests := []struct {
		label string
		input telemetryv1beta1.MetricPipelineInput
	}{
		{
			label: suite.LabelMetricAgentSetA,
			input: testutils.BuildMetricPipelineRuntimeInput(),
		},
		{
			label: suite.LabelMetricGatewaySetA,
			input: testutils.BuildMetricPipelineOTLPInput(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
			)

			metricPipelineWithGRPCAndWithoutPath := testutils.NewMetricPipelineBuilder().
				WithName(uniquePrefix("accept-with-grpc-and-no-path")).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("grpc")).
				Build()

			metricPipelineWithHTTPAndPath := testutils.NewMetricPipelineBuilder().
				WithName(uniquePrefix("accept-with-http-and-path")).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("http")).
				Build()

			metricPipelineWithHTTPAndWithoutPath := testutils.NewMetricPipelineBuilder().
				WithName(uniquePrefix("accept-with-http-and-no-path")).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("http")).
				Build()

			resources := []client.Object{
				&metricPipelineWithGRPCAndWithoutPath,
				&metricPipelineWithHTTPAndPath,
				&metricPipelineWithHTTPAndWithoutPath,
			}

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
		})
	}
}
