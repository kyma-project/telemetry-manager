//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
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
				Expect(kitk8s.DeleteObjects(ctx, k8sClient,
					&tracePipelineWithGRPCAndWithoutPath, &tracePipelineWithHTTPAndPath, &tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a TracePipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a TracePipeline with path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a TracePipeline with no path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with no path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
