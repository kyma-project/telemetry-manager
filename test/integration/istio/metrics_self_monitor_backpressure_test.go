//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringMetricsBackpressure), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics, kitbackend.WithAbortFaultInjection(85))
		objs = append(objs, backend.K8sObjects()...)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		objs = append(objs,
			&metricPipeline,
			telemetrygen.NewDeployment(mockNs, telemetrygen.SignalTypeMetrics,
				telemetrygen.WithRate(800),
				telemetrygen.WithWorkers(5)).K8sObject(),
		)

		return objs
	}

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metricpipeline", func() {
			assert.MetricPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.SelfMonitorName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
		})

		It("Should have a telemetrygen running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: telemetrygen.DefaultName, Namespace: mockNs})
		})

		It("Should wait for the metrics flow to gradually become unhealthy", func() {
			assert.MetricPipelineConditionReasonsTransition(suite.Ctx, suite.K8sClient, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
				{Reason: conditions.ReasonSelfMonSomeDataDropped, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(suite.Ctx, suite.K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeMetricComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonSomeDataDropped,
			})
		})
	})
})
