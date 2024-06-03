//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringTracesBackpressure), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := backend.New(mockNs, backend.SignalTypeTraces, backend.WithAbortFaultInjection(95))
		objs = append(objs, backend.K8sObjects()...)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()

		objs = append(objs,
			&tracePipeline,
			telemetrygen.NewDeployment(mockNs, telemetrygen.SignalTypeTraces,
				telemetrygen.WithRate(800),
				telemetrygen.WithWorkers(5)).K8sObject(),
		)

		return objs
	}

	Context("Before deploying a tracepipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running tracepipeline", func() {
			assert.TracePipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a telemetrygen running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: telemetrygen.DefaultName, Namespace: mockNs})
		})

		It("Should wait for the trace flow to gradually become unhealthy", func() {
			assert.TracePipelineConditionReasonsTransition(ctx, k8sClient, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
				{Reason: conditions.ReasonSelfMonSomeDataDropped, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasWarningState(ctx, k8sClient)
			assert.TelemetryHasCondition(ctx, k8sClient, metav1.Condition{
				Type:   conditions.TypeTraceComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonSomeDataDropped,
			})
		})
	})
})
