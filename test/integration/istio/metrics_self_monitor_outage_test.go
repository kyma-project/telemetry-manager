//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringMetricsOutage), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithReplicas(0))
		objs = append(objs, backend.K8sObjects()...)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		objs = append(objs,
			&metricPipeline,
			telemetrygen.NewDeployment(mockNs, telemetrygen.SignalTypeMetrics, telemetrygen.WithRate(10_000_000), telemetrygen.WithWorkers(50), telemetrygen.WithInterval("30s")).WithReplicas(2).K8sObject(),
		)

		return objs
	}

	Context("Before deploying a metricpipeline", func() {
		It("Should set scaling for metrics", Label(suite.LabelOperational), func() {
			// retry until the Telemetry CR is updated correctly
			Eventually(func() error {
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
				if err != nil {
					return err
				}

				telemetry.Spec.Metric = &operatorv1alpha1.MetricSpec{
					Gateway: operatorv1alpha1.MetricGatewaySpec{
						Scaling: operatorv1alpha1.Scaling{
							Type: operatorv1alpha1.StaticScalingStrategyType,
							Static: &operatorv1alpha1.StaticScaling{
								Replicas: 1,
							},
						},
					},
				}
				err = k8sClient.Update(ctx, &telemetry)
				return err
			}, "1m", "10s").Should(Succeed())

		})

		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metricpipeline", func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a telemetrygen running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: telemetrygen.DefaultName, Namespace: mockNs})
		})

		It("Should wait for the metrics flow to report a full buffer", func() {
			assert.MetricPipelineConditionReasonsTransition(ctx, k8sClient, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
				{Reason: conditions.ReasonSelfMonBufferFillingUp, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(ctx, k8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonBufferFillingUp,
			})
		})

		// this is needed to give the metrics flow time to report a full buffer
		It("Should stop sending metrics from telemetrygen", func() {
			var telgen v1.Deployment
			err := k8sClient.Get(ctx, types.NamespacedName{Namespace: mockNs, Name: telemetrygen.DefaultName}, &telgen)
			Expect(err).NotTo(HaveOccurred())

			telgen.Spec.Replicas = ptr.To(int32(0))
			err = k8sClient.Update(ctx, &telgen)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should wait for the metrics flow to report dropped metrics", func() {
			assert.MetricPipelineConditionReasonsTransition(ctx, k8sClient, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonAllDataDropped, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(ctx, k8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonAllDataDropped,
			})
		})
	})
})
