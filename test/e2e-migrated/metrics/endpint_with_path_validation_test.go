package metrics

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestEndpointWithPathValidation(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

	metricPipelineWithGRPCAndWithoutPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-accept-with-grpc-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("grpc")).
		Build()

	metricPipelineWithHTTPAndPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-accept-with-http-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("http")).
		Build()

	metricPipelineWithHTTPAndWithoutPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-accept-with-http-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("http")).
		Build()

	resources := []client.Object{
		&metricPipelineWithGRPCAndWithoutPath,
		&metricPipelineWithHTTPAndPath,
		&metricPipelineWithHTTPAndWithoutPath,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(t, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())
}
