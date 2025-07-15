package traces

import (
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

func TestEndpointWithPathValidation(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	tracePipelineDefaultGRPCWithPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-default-reject-with-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/traces")).
		Build()

	tracePipelineWithGRPCAndWithoutPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-grpc-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("grpc")).
		Build()

	tracePipelineWithGRPCAndPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-reject-with-grpc-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/traces"), testutils.OTLPProtocol("grpc")).
		Build()

	tracePipelineWithHTTPAndPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-http-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("http")).
		Build()

	tracePipelineWithHTTPAndWithoutPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-http-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("http")).
		Build()

	resources := []client.Object{
		&tracePipelineWithGRPCAndWithoutPath,
		&tracePipelineWithHTTPAndPath,
		&tracePipelineWithHTTPAndWithoutPath,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(resources...))
	})
	
	Expect(kitk8s.CreateObjects(t, resources...)).Should(Succeed())

	Expect(kitk8s.CreateObjects(t, &tracePipelineWithGRPCAndPath)).ShouldNot(Succeed())
	Expect(kitk8s.CreateObjects(t, &tracePipelineDefaultGRPCWithPath)).ShouldNot(Succeed())

}
