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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/floggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringLogsFluentBitBackpressure), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeLogsFluentBit, kitbackend.WithAbortFaultInjection(85))
		logProducer := floggen.NewDeployment(mockNs).WithReplicas(3)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("When a logpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			assert.FluentBitLogPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(suite.Ctx, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.SelfMonitorName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: kitbackend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Namespace: mockNs, Name: floggen.DefaultName})
		})

		It("Should wait for the log flow to gradually become unhealthy", func() {
			assert.LogPipelineConditionReasonsTransition(suite.Ctx, pipelineName, conditions.TypeFlowHealthy, []assert.ReasonStatus{
				{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
				{Reason: conditions.ReasonSelfMonAgentBufferFillingUp, Status: metav1.ConditionFalse},
				{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
			})

			assert.TelemetryHasState(suite.Ctx, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(suite.Ctx, suite.K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
			})
		})
	})
})
