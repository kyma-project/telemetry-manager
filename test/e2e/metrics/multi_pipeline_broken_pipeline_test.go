//go:build e2e

package metrics

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

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetC), Ordered, func() {
	Context("When a broken metricpipeline exists", Ordered, func() {
		var (
			mockNs              = ID()
			healthyPipelineName = IDWithSuffix("healthy")
			brokenPipelineName  = IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeMetrics)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(ProxyClient)

			healthyPipeline := testutils.NewMetricPipelineBuilder().
				WithName(healthyPipelineName).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()
			objs = append(objs, &healthyPipeline)

			endpointKey := "metric-endpoint"
			unreachableHostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData(endpointKey, "http://unreachable:4317"))
			brokenPipeline := testutils.NewMetricPipelineBuilder().
				WithName(brokenPipelineName).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(unreachableHostSecret.Name(), unreachableHostSecret.Namespace(), endpointKey)).
				Build()
			objs = append(objs, &brokenPipeline, unreachableHostSecret.K8sObject())

			objs = append(objs,
				telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			)
			return objs
		}

		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.MetricPipelineHealthy(Ctx, K8sClient, healthyPipelineName)
			assert.MetricPipelineHealthy(Ctx, K8sClient, brokenPipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should deliver telemetrygen metrics", func() {
			assert.MetricsFromNamespaceDelivered(ProxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})
	})
})
