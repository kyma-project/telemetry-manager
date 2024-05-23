//go:build istio

package istio

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelSelfMonitoringLogsBackpressure), Ordered, func() {
	var (
		mockNs       = "istio-permissive-mtls"
		pipelineName = suite.ID()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		backend := backend.New(mockNs, backend.SignalTypeLogs, backend.WithAbortFaultInjection(90))
		logProducer := loggen.New(mockNs).WithReplicas(3).WithLoad(loggen.LoadHigh)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())

		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running logpipeline", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a running self-monitor", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: backend.DefaultName})
		})

		It("Should have a log producer running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNs, Name: loggen.DefaultName})
		})

		It("Should wait for the log flow to gradually become unhealthy", func() {
			assert.LogPipelineConditionReasonsTransition(ctx, k8sClient, pipelineName, conditions.TypeFlowHealthy, []string{
				conditions.ReasonSelfMonFlowHealthy,
				conditions.ReasonSelfMonBufferFillingUp,
				conditions.ReasonSelfMonSomeDataDropped,
			})
		})
	})
})
