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

	Context("When a broken tracepipeline exists", Ordered, func() {
		var (
			mockNs              = ID()
			healthyPipelineName = IDWithSuffix("healthy")
			brokenPipelineName  = IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeTraces)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(ProxyClient)

			healthyPipeline := testutils.NewTracePipelineBuilder().
				WithName(healthyPipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()
			objs = append(objs, &healthyPipeline)

			endpointKey := "trace-endpoint"
			unreachableHostSecret := kitk8s.NewOpaqueSecret("unreachable", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData(endpointKey, "http://unreachable:4317"))
			brokenPipeline := testutils.NewTracePipelineBuilder().
				WithName(brokenPipelineName).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(unreachableHostSecret.Name(), unreachableHostSecret.Namespace(), endpointKey)).
				Build()
			objs = append(objs, &brokenPipeline, unreachableHostSecret.K8sObject())

			objs = append(objs,
				telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).K8sObject(),
			)
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
			assert.TracePipelineHealthy(Ctx, K8sClient, healthyPipelineName)
			assert.TracePipelineHealthy(Ctx, K8sClient, brokenPipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should verify traces from telemetrygen are delivered", func() {
			assert.TracesFromNamespaceDelivered(ProxyClient, backendExportURL, mockNs)
		})
	})
})
