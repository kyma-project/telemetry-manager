//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetB), Ordered, func() {
	metricPipelineDefaultGRPCWithPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-default-reject-with-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics")).
		Build()

	metricPipelineWithGRPCAndPath := testutils.NewMetricPipelineBuilder().
		WithName("metricpipeline-reject-with-grpc-and-path").
		WithOTLPOutput(testutils.OTLPEndpoint("mock-endpoint:4817"), testutils.OTLPEndpointPath("/v1/mock/metrics"), testutils.OTLPProtocol("grpc")).
		Build()

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

	Context("When a MetricPipeline sets endpoint path", Ordered, func() {

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient,
					&metricPipelineWithGRPCAndWithoutPath, &metricPipelineWithHTTPAndPath, &metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
			})
		})

		It("Should reject a MetricPipeline with path and default protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipelineDefaultGRPCWithPath)).ShouldNot(Succeed())
		})

		It("Should reject a MetricPipeline with path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipelineWithGRPCAndPath)).ShouldNot(Succeed())
		})

		It("Should accept a MetricPipeline with no path and gRPC protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipelineWithGRPCAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a MetricPipeline with no path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipelineWithHTTPAndWithoutPath)).Should(Succeed())
		})

		It("Should accept a MetricPipeline with path and HTTP protocol", func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipelineWithHTTPAndPath)).Should(Succeed())
		})
	})
})
