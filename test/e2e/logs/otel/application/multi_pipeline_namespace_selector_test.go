//go:build e2e

package application

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelSignalPull, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs                    = suite.ID()
		app1Ns                    = suite.IDWithSuffix("app-1")
		app2Ns                    = suite.IDWithSuffix("app-2")
		backendNs1                = suite.IDWithSuffix("backend-1")
		backendNs2                = suite.IDWithSuffix("backend-2")
		backend1Name              = "backend-1"
		backend1ExportURL         string
		backend2Name              = "backend-2"
		backend2ExportURL         string
		pipelineNameIncludeApp1Ns = "include-" + app1Ns
		pipelineNameExcludeApp1Ns = "exclude-" + app1Ns
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns).K8sObject(),
			kitk8s.NewNamespace(app2Ns).K8sObject(),
			kitk8s.NewNamespace(backendNs1).K8sObject(),
			kitk8s.NewNamespace(backendNs2).K8sObject())

		backend1 := backend.New(backendNs1, backend.SignalTypeLogsOtel, backend.WithName(backend1Name))
		backend1ExportURL = backend1.ExportURL(suite.ProxyClient)
		objs = append(objs, backend1.K8sObjects()...)

		pipelineIncludeApp1Ns := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameIncludeApp1Ns).
			WithOTLPInput(false).
			WithApplicationInput(true, testutils.IncludeLogNamespaces(app1Ns)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
			Build()
		objs = append(objs, &pipelineIncludeApp1Ns)

		backend2 := backend.New(backendNs2, backend.SignalTypeLogsOtel, backend.WithName(backend2Name))
		backend2ExportURL = backend2.ExportURL(suite.ProxyClient)
		objs = append(objs, backend2.K8sObjects()...)

		pipelineExcludeApp1Ns := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameExcludeApp1Ns).
			WithOTLPInput(false).
			WithApplicationInput(true, testutils.IncludeLogNamespaces(app2Ns)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
			Build()
		objs = append(objs, &pipelineExcludeApp1Ns)

		logProducer1 := loggen.New(app1Ns)
		logProducer2 := loggen.New(app2Ns)

		objs = append(objs,
			logProducer1.K8sObject(),
			logProducer2.K8sObject(),
		)

		return objs
	}

	Context("When multiple logpipelines exist", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})

		It("Should have a logs backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: backendNs1})
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: backendNs2})
			assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: backendNs1})
			assert.ServiceReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: backendNs2})
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineOtelHealthy(suite.Ctx, suite.K8sClient, pipelineNameIncludeApp1Ns)
			assert.LogPipelineOtelHealthy(suite.Ctx, suite.K8sClient, pipelineNameExcludeApp1Ns)
		})

		// verify logs from apps1Ns delivered to backend1
		It("Should deliver Runtime logs from app1Ns to backend1", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, app1Ns)
		})

		It("Should not deliver logs from app2Ns to backend1", func() {
			assert.OtelLogsFromNamespaceNotDelivered(suite.ProxyClient, backend1ExportURL, app2Ns)
		})

		// verify logs from apps2Ns delivered to backend2
		It("Should deliver Runtime logs from app2Ns to backend2", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, app2Ns)
		})

		It("Should not deliver logs from app1Ns to backend2", func() {
			assert.OtelLogsFromNamespaceNotDelivered(suite.ProxyClient, backend2ExportURL, app1Ns)
		})
	})
})
