//go:build e2e

package traces

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
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTraces), Ordered, func() {
	Context("When multiple tracepipelines exist", Ordered, func() {
		var (
			mockNs            = ID()
			backend1Name      = IDWithSuffix("backend-1")
			pipeline1Name     = IDWithSuffix("1")
			backend1ExportURL string
			backend2Name      = IDWithSuffix("backend-2")
			pipeline2Name     = IDWithSuffix("2")
			backend2ExportURL string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend1 := backend.New(mockNs, backend.SignalTypeTraces, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(ProxyClient)

			tracePipeline1 := testutils.NewTracePipelineBuilder().
				WithName(pipeline1Name).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()
			objs = append(objs, &tracePipeline1)

			backend2 := backend.New(mockNs, backend.SignalTypeTraces, backend.WithName(backend2Name))
			objs = append(objs, backend2.K8sObjects()...)
			backend2ExportURL = backend2.ExportURL(ProxyClient)

			tracePipeline2 := testutils.NewTracePipelineBuilder().
				WithName(pipeline2Name).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()
			objs = append(objs, &tracePipeline2)

			objs = append(objs, telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).K8sObject())
			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.TracePipelineHealthy(Ctx, K8sClient, pipeline1Name)
			assert.TracePipelineHealthy(Ctx, K8sClient, pipeline2Name)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should verify traces from telemetrygen are delivered", func() {
			assert.TracesFromNamespaceDelivered(ProxyClient, backend1ExportURL, mockNs)
			assert.TracesFromNamespaceDelivered(ProxyClient, backend2ExportURL, mockNs)
		})
	})
})
