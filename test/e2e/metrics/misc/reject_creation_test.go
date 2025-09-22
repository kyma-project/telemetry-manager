package misc

import (
	"testing"

	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRejectPipelineCreation(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetC)

	defaultGRPCWithPathPipeline := testutils.NewMetricPipelineBuilder().
		WithName("default-reject-with-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics")).
		Build()

	withGRPCAndPathPipeline := testutils.NewMetricPipelineBuilder().
		WithName("reject-with-grpc-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("grpc")).
		Build()

	misconfiguredSecretRefPipeline := testutils.NewMetricPipelineBuilder().
		WithName("misconfigured-secretref").
		WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("name", "namespace", "", "")).
		Build()

	Expect(kitk8s.CreateObjects(t, &defaultGRPCWithPathPipeline)).ShouldNot(Succeed())
	Expect(kitk8s.CreateObjects(t, &withGRPCAndPathPipeline)).ShouldNot(Succeed())
	Expect(kitk8s.CreateObjects(t, &misconfiguredSecretRefPipeline)).ShouldNot(Succeed())
}
