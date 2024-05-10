//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	metricPipelineDefaultGRPCWithPath := kitk8s.NewMetricPipelineV1Alpha1("metricpipeline-default-reject-with-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		Persistent(suite.IsOperational()).K8sObject()

	metricPipelineWithGRPCAndPath := kitk8s.NewMetricPipelineV1Alpha1("metricpipeline-reject-with-grpc-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		WithProtocol("grpc").
		Persistent(suite.IsOperational()).K8sObject()

	metricPipelineWithGRPCAndWithoutPath := kitk8s.NewMetricPipelineV1Alpha1("metricpipeline-accept-with-grpc-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("grpc").
		Persistent(suite.IsOperational()).K8sObject()

	metricPipelineWithHTTPAndPath := kitk8s.NewMetricPipelineV1Alpha1("metricpipeline-accept-with-http-and-path").
		WithOutputEndpoint("mock-endpoint:4817").WithEndpointPath("/v1/mock/metrics").
		WithProtocol("http").
		Persistent(suite.IsOperational()).K8sObject()

	metricPipelineWithHTTPAndWithoutPath := kitk8s.NewMetricPipelineV1Alpha1("metricpipeline-accept-with-http-and-no-path").
		WithOutputEndpoint("mock-endpoint:4817").
		WithProtocol("http").
		Persistent(suite.IsOperational()).K8sObject()

	Context("When a MetricPipeline sets endpoint path", Ordered, func() {

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient,
					metricPipelineWithGRPCAndWithoutPath, metricPipelineWithHTTPAndPath, metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a MetricPipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a MetricPipeline with path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a MetricPipeline with no path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a MetricPipeline with no path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a MetricPipeline with path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
