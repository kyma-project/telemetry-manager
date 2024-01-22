//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

var _ = Describe("Traces Validating Endpoint Path", Label("tracing"), Ordered, func() {

	tracePipelineDefaultGRPCWithPath := kitk8s.NewTracePipeline("tracepipeline-default-reject-with-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/traces").
		Persistent(isOperational()).K8sObject()

	tracePipelineWithGRPCAndPath := kitk8s.NewTracePipeline("tracepipeline-reject-with-grpc-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/traces").
		WithProtocol("grpc").
		Persistent(isOperational()).K8sObject()

	tracePipelineWithGRPCAndWithoutPath := kitk8s.NewTracePipeline("tracepipeline-accept-with-grpc-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("grpc").
		Persistent(isOperational()).K8sObject()

	tracePipelineWithHTTPAndPath := kitk8s.NewTracePipeline("tracepipeline-accept-with-http-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/traces").
		WithProtocol("http").
		Persistent(isOperational()).K8sObject()

	tracePipelineWithHTTPAndWithoutPath := kitk8s.NewTracePipeline("tracepipeline-accept-with-http-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("http").
		Persistent(isOperational()).K8sObject()

	Context("When a trace pipeline set endpoint path", Ordered, func() {

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient,
					tracePipelineWithGRPCAndWithoutPath, tracePipelineWithHTTPAndPath, tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a TracePipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a TracePipeline with path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a TracePipeline with no path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with no path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a TracePipeline with path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
