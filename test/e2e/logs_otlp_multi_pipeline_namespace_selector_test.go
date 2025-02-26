//go:build e2e

package e2e

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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Label(suite.LabelSetC), Ordered, func() {
	var (
		mockNs            = suite.ID()
		app1Ns            = "app-1"
		app2Ns            = "app-2"
		backend1Name      = "backend-1"
		backend1ExportURL string
		backend2Name      = "backend-2"
		backend2ExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(app1Ns).K8sObject(),
			kitk8s.NewNamespace(app2Ns).K8sObject())

		backend1 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend1Name))
		backend1ExportURL = backend1.ExportURL(proxyClient)
		objs = append(objs, backend1.K8sObjects()...)

		pipelineIncludeApp1Ns := testutils.NewLogPipelineBuilder().
			WithName("include-"+app1Ns).
			WithApplicationInput(true, testutils.IncludeLogNamespaces(app1Ns)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
			Build()
		objs = append(objs, &pipelineIncludeApp1Ns)

		backend2 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend2Name))
		backend2ExportURL = backend2.ExportURL(proxyClient)
		objs = append(objs, backend2.K8sObjects()...)

		pipelineExcludeApp1Ns := testutils.NewLogPipelineBuilder().
			WithName("exclude-"+app1Ns).
			WithApplicationInput(true, testutils.ExcludeLogNamespaces(app1Ns)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
			Build()
		objs = append(objs, &pipelineExcludeApp1Ns)

		objs = append(objs,
			telemetrygen.NewPod(app1Ns, telemetrygen.SignalTypeLogs).K8sObject(),
			telemetrygen.NewPod(app2Ns, telemetrygen.SignalTypeLogs).K8sObject(),
			telemetrygen.NewPod(kitkyma.SystemNamespaceName, telemetrygen.SignalTypeLogs).K8sObject(),
		)

		return objs
	}

	Context("When multiple logpipelines exist", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.LogAgentName)
		})

		It("Should have a logs backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.ServiceReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		// FIXME
		// verify logs from apps1Ns delivered to backend1
		It("Should deliver Runtime logs from app1Ns to backend1", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backend1ExportURL, app1Ns)
		})

		It("Should deliver OTLP logs from app1Ns to backend1", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backend1ExportURL, app1Ns)
		})

		It("Should not deliver logs from app2Ns to backend1", func() {
			assert.LogsFromNamespaceNotDelivered(proxyClient, backend1ExportURL, app2Ns)
		})

		// FIXME
		// verify logs from apps2Ns delivered to backend2
		It("Should deliver Runtime logs from app2Ns to backend2", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backend2ExportURL, app2Ns)
		})

		It("Should deliver OTLP logs from app2Ns to backend2", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backend2ExportURL, app2Ns)
		})

		It("Should not deliver logs from app1Ns to backend2", func() {
			assert.LogsFromNamespaceNotDelivered(proxyClient, backend2ExportURL, app1Ns)
		})
	})
})
