//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringTracesOutage), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeTraces, kitbackend.WithReplicas(0))
		objs = append(objs, backend.K8sObjects()...)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		objs = append(objs,
			&tracePipeline,
			telemetrygen.NewDeployment(mockNs, telemetrygen.SignalTypeTraces,
				telemetrygen.WithRate(80),
				telemetrygen.WithWorkers(10)).K8sObject(),
		)

		return objs
	}

	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running tracepipeline", func() {
			assert.TracePipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.TraceGatewayName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.SelfMonitorName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
		})

		It("Should have a telemetrygen running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: telemetrygen.DefaultName, Namespace: mockNs})
		})

		It("Should wait for the trace flow to report a full buffer", func() {
			assert.TracePipelineConditionReasonsTransition(suite.Ctx, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
				{Reason: conditions.ReasonSelfMonGatewayBufferFillingUp, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(suite.Ctx, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeTraceComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonGatewayBufferFillingUp,
			})
		})

		It("Should stop sending metrics from telemetrygen", func() {
			var telgen appsv1.Deployment
			err := suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: telemetrygen.DefaultName}, &telgen)
			Expect(err).NotTo(HaveOccurred())

			telgen.Spec.Replicas = ptr.To(int32(0))
			err = suite.K8sClient.Update(suite.Ctx, &telgen)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should wait for the trace flow to report dropped metrics", func() {
			assert.TracePipelineConditionReasonsTransition(suite.Ctx, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(suite.Ctx, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeTraceComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
			})
		})

		Context("Metric instrumentation", Ordered, func() {
			It("Ensures that controller_runtime_webhook_requests_total is increased", func() {
				// Pushing metrics to the metric gateway triggers an alert.
				// It makes the self-monitor call the webhook, which in turn increases the counter.
				assert.EmitsManagerMetrics(suite.Ctx,
					HaveName(Equal("controller_runtime_webhook_requests_total")),
					SatisfyAll(
						HaveLabels(HaveKeyWithValue("webhook", "/api/v2/alerts")),
						HaveMetricValue(BeNumerically(">", 0)),
					))
			})

			It("Ensures that telemetry_self_monitor_prober_requests_total is emitted", func() {
				assert.EmitsManagerMetrics(suite.Ctx,
					HaveName(Equal("telemetry_self_monitor_prober_requests_total")),
					HaveMetricValue(BeNumerically(">", 0)),
				)
			})
		})
	})
})
