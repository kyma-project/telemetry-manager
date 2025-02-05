//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
		mockNs       = suite.ID()
		pipelineName = suite.ID()
	)

	makeResourcesNoInput := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)

		metricPipelineNoInput := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithPrometheusInput(false).
			WithRuntimeInput(false).
			WithIstioInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipelineNoInput)

		return objs
	}

	Context("When a metricpipeline with no input enabled exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResourcesNoInput()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", Label(suite.LabelUpgrade), func() {
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		// TODO: Pipeline should have AgentRunning condition
		// TODO: DaemonSet is running
		// TODO: Disable inputs

		It("Pipeline should have AgentNotRequired condition", Label(suite.LabelUpgrade), func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeAgentHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonMetricAgentNotRequired,
			})
		})

		It("Ensures the metric agent DaemonSet is not running", func() {
			assert.DaemonSetNotFound(ctx, k8sClient, kitkyma.MetricAgentName)
		})
	})
})
