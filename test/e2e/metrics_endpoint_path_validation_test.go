//go:build e2e

package e2e

import (
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitmetricpipeline "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metrics Validating Endpoint Path", Label("metrics"), Ordered, func() {

	metricPipelineDefaultGRPCWithPath := kitmetricpipeline.NewPipeline("metricpipeline-default-reject-with-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		Persistent(isOperational()).K8sObject()

	metricPipelineWithGRPCAndPath := kitmetricpipeline.NewPipeline("metricpipeline-reject-with-grpc-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		WithProtocol("grpc").
		Persistent(isOperational()).K8sObject()

	metricPipelineWithGRPCAndWithoutPath := kitmetricpipeline.NewPipeline("metricpipeline-accept-with-grpc-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("grpc").
		Persistent(isOperational()).K8sObject()

	metricPipelineWithHTTPAndPath := kitmetricpipeline.NewPipeline("metricpipeline-accept-with-http-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		WithProtocol("http").
		Persistent(isOperational()).K8sObject()

	metricPipelineWithHTTPAndWithoutPath := kitmetricpipeline.NewPipeline("metricpipeline-accept-with-http-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("http").
		Persistent(isOperational()).K8sObject()

	Context("When a metric pipeline set endpoint path", Ordered, func() {

		AfterAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient,
					metricPipelineWithGRPCAndWithoutPath, metricPipelineWithHTTPAndPath, metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a metricpipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a metricpipeline with path and grpc protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a metricpipeline with no path and grpc protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a metricpipeline with no path and http protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a metricpipeline with path and http protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
