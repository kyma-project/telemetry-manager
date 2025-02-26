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

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("When multiple logpipelines exist", Ordered, func() {
		var (
			mockNs            = suite.ID()
			backend1Name      = suite.IDWithSuffix("backend-1")
			pipeline1Name     = suite.IDWithSuffix("1")
			backend1ExportURL string
			backend2Name      = suite.IDWithSuffix("backend-2")
			pipeline2Name     = suite.IDWithSuffix("2")
			backend2ExportURL string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend1 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(proxyClient)

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithApplicationInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()
			objs = append(objs, &logPipeline1)

			backend2 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend2Name))
			objs = append(objs, backend2.K8sObjects()...)
			backend2ExportURL = backend2.ExportURL(proxyClient)

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithApplicationInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()
			objs = append(objs, &logPipeline2)

			objs = append(objs, telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeLogs).K8sObject())
			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipeline1Name)
			assert.LogPipelineHealthy(ctx, k8sClient, pipeline2Name)
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.LogAgentName)
		})

		It("Should have running backends", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		// FIXME
		It("Should verify logs from telemetrygen are delivered", func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backend1ExportURL, mockNs)
			assert.LogsFromNamespaceDelivered(proxyClient, backend2ExportURL, mockNs)
		})
	})
})
