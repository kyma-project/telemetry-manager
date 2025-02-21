//go:build e2e

package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetC), Ordered, func() {
	Context("When a metricpipeline with missing secret reference exists", Ordered, func() {
		var pipelineName = ID()

		endpointKey := "metrics-endpoint"
		secret := kitk8s.NewOpaqueSecret("metrics-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &metricPipeline)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &metricPipeline, secret.K8sObject())).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to False", func() {
			assert.MetricPipelineHasCondition(Ctx, K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should set TelemetryFlowHealthy condition to False", func() {
			assert.MetricPipelineHasCondition(Ctx, K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should set MetricComponentsHealthy condition to False in Telemetry", func() {
			assert.TelemetryHasState(Ctx, K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(Ctx, K8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should not have metric gateway deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				err := K8sClient.Get(Ctx, kitkyma.MetricGatewayName, &deployment)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Deployment still exists")
		})

		It("Should have running metricpipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(Ctx, K8sClient, secret.K8sObject())).Should(Succeed())
			})

			assert.MetricPipelineHealthy(Ctx, K8sClient, pipelineName)
		})
	})
})
