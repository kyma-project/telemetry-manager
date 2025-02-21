//go:build e2e

package traces

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTraces), func() {
	tracePipelineDefaultGRPCWithPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-default-reject-with-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/traces")).
		Build()

	tracePipelineWithGRPCAndPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-reject-with-grpc-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/traces"), testutils.OTLPProtocol("grpc")).
		Build()

	tracePipelineWithGRPCAndWithoutPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-grpc-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("grpc")).
		Build()

	tracePipelineWithHTTPAndPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-http-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/traces"), testutils.OTLPProtocol("http")).
		Build()

	tracePipelineWithHTTPAndWithoutPath := testutils.NewTracePipelineBuilder().
		WithName("tracepipeline-accept-with-http-and-no-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPProtocol("http")).
		Build()

	Context("When a trace pipeline set endpoint path", Ordered, func() {

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient,
					&tracePipelineWithGRPCAndWithoutPath, &tracePipelineWithHTTPAndPath, &tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a TracePipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a TracePipeline with path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a TracePipeline with no path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with no path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
