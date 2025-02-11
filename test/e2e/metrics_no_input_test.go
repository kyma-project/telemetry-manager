//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetA), Ordered, func() {
	var (
		mockNs                = suite.ID()
		pipelineNameNoInput   = suite.ID() + "-no-input"
		pipelineNameWithInput = suite.ID() + "-with-input"
	)

	var metricPipelineWithInput telemetryv1alpha1.MetricPipeline

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)

		metricPipelineNoInput := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameNoInput).
			WithPrometheusInput(false).
			WithRuntimeInput(false).
			WithIstioInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipelineNoInput)
		metricPipelineWithInput = testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameWithInput).
			WithPrometheusInput(true).
			WithRuntimeInput(true).
			WithIstioInput(true).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		return objs
	}

	Context("When a metricpipeline with no input enabled exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			k8sObjectsToCreate := append(k8sObjects, &metricPipelineWithInput)
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjectsToCreate...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have running pipelines", Label(suite.LabelUpgrade), func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineNameNoInput)
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineNameWithInput)
		})

		It("Pipeline with no input should have AgentNotRequired condition", Label(suite.LabelUpgrade), func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineNameNoInput, metav1.Condition{
				Type:   conditions.TypeAgentHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonMetricAgentNotRequired,
			})
		})

		It("Ensures the metric agent DaemonSet is running", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should delete the pipeline with input", func() {
			Expect(kitk8s.DeleteObjects(ctx, k8sClient, &metricPipelineWithInput)).Should(Succeed())
		})

		It("Ensures the metric agent DaemonSet is no longer running", func() {
			assert.DaemonSetNotFound(ctx, k8sClient, kitkyma.MetricAgentName)
		})
	})
})
