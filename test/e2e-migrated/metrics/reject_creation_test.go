package metrics

import (
	"testing"

	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectPipelineCreation(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	metricPipelineDefaultGRPCWithPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-default-reject-with-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics")).
		Build()

	metricPipelineWithGRPCAndPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-reject-with-grpc-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("grpc")).
		Build()

	Expect(kitk8s.CreateObjects(suite.Ctx, &metricPipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
	Expect(kitk8s.CreateObjects(suite.Ctx, &metricPipelineWithGRPCAndPath)).ShouldNot(Succeed())
}
