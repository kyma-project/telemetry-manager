//go:build e2e

package metrics

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
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics, suite.LabelSetA), Ordered, func() {
	var (
		mockNs                = suite.ID()
		pipelineNameNoInput   = suite.ID() + "-no-input"
		pipelineNameWithInput = suite.ID() + "-with-input"
	)

	var metricPipelineWithInput telemetryv1alpha1.MetricPipeline

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
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
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})

			k8sObjectsToCreate := append(k8sObjects, &metricPipelineWithInput)
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjectsToCreate...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have running pipelines", func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineNameNoInput)
			assert.MetricPipelineHealthy(suite.Ctx, pipelineNameWithInput)
		})

		It("Pipeline with no input should have AgentNotRequired condition", func() {
			assert.MetricPipelineHasCondition(suite.Ctx, pipelineNameNoInput, metav1.Condition{
				Type:   conditions.TypeAgentHealthy,
				Status: metav1.ConditionTrue,
				Reason: conditions.ReasonMetricAgentNotRequired,
			})
		})

		It("Ensures the metric agent DaemonSet is running", func() {
			assert.DaemonSetReady(suite.Ctx, kitkyma.MetricAgentName)
		})

		It("Should delete the pipeline with input", func() {
			Expect(kitk8s.DeleteObjects(suite.Ctx, &metricPipelineWithInput)).Should(Succeed())
		})

		It("Ensures the metric agent DaemonSet is no longer running", func() {
			assert.DaemonSetNotFound(suite.Ctx, kitkyma.MetricAgentName)
		})
	})
})
