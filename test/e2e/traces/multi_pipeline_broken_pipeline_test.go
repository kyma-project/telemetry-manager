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
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), Ordered, func() {

	Context("When a broken tracepipeline exists", Ordered, func() {
		var (
			mockNs              = suite.ID()
			healthyPipelineName = suite.IDWithSuffix("healthy")
			brokenPipelineName  = suite.IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := kitbackend.New(mockNs, kitbackend.SignalTypeTraces)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(suite.ProxyClient)

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
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.TracePipelineHealthy(suite.Ctx, suite.K8sClient, healthyPipelineName)
			assert.TracePipelineHealthy(suite.Ctx, suite.K8sClient, brokenPipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should verify traces from telemetrygen are delivered", func() {
			assert.TracesFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})
	})
})
