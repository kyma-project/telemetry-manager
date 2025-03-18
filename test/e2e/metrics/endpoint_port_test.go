//go:build e2e

package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetC), func() {
	const (
		invalidPortEndpoint = "http://example.com:9abc8"
		missingPortEndpoint = "http://example.com:/"
	)

	var (
		mockNs                  = ID()
		pipelineNameInvalid     = IDWithSuffix("invalid")
		pipelineNameMissingGRPC = IDWithSuffix("missing-grpc")
		pipelineNameMissingHTTP = IDWithSuffix("missing-http")
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		metricPipelineInvalidPort := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameInvalid).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidPortEndpoint),
			).
			Build()

		metricPipelineMissingPortGRPC := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameMissingGRPC).
			WithOTLPOutput(
				testutils.OTLPEndpoint(missingPortEndpoint),
			).
			Build()

		metricPipelineMissingPortHTTP := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameMissingHTTP).
			WithOTLPOutput(
				testutils.OTLPEndpoint(missingPortEndpoint),
				testutils.OTLPProtocol("http"),
			).
			Build()

		objs = append(objs,
			&metricPipelineInvalidPort,
			&metricPipelineMissingPortGRPC,
			&metricPipelineMissingPortHTTP,
		)

		return objs
	}

	Context("When metric pipelines with an invalid or missing Endpoint port are created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.MetricPipelineHasCondition(Ctx, K8sClient, pipelineNameInvalid, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.MetricPipelineHasCondition(Ctx, K8sClient, pipelineNameMissingGRPC, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})

		It("Should set ConfigurationGenerated condition to True in pipelines with missing port and HTTP protocol", func() {
			assert.MetricPipelineHasCondition(Ctx, K8sClient, pipelineNameMissingHTTP, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonGatewayConfigured,
			})
		})
	})
})
