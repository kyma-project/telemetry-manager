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
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), func() {
	Context("When metricpipeline referecing a secret with incorrect endpoint exists", Ordered, func() {
		const endpointKey = "metrics-endpoint"

		var (
			mockNs       = suite.ID()
			pipelineName = suite.ID()
			backend      *kitbackend.Backend
		)

		// Initially, create a secret with an incorrect endpoint
		secret := kitk8s.NewOpaqueSecret("metrics-secret-rotation", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4000"))
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()
		backend = kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)

		resources := []client.Object{
			kitk8s.NewNamespace(mockNs).K8sObject(),
			secret.K8sObject(),
			&metricPipeline,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
		}
		resources = append(resources, backend.K8sObjects()...)

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(suite.Ctx, resources...)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, resources...)).Should(Succeed())
			})
		})

		It("Should have a running metrics gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should initially not deliver telemetrygen metrics to the backend due to the incorrect endpoint in the secret", func() {
			assert.MetricsFromNamespaceNotDelivered(suite.ProxyClient, backend.ExportURL(suite.ProxyClient), mockNs)
		})

		It("Should deliver telemetrygen metrics to the backend", func() {
			By("Updating secret with the correct endpoint", func() {
				secret.UpdateSecret(kitk8s.WithStringData(endpointKey, backend.Endpoint()))
				Expect(kitk8s.UpdateObjects(suite.Ctx, secret.K8sObject())).Should(Succeed())
			})

			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)

			assert.MetricsFromNamespaceDelivered(suite.ProxyClient, backend.ExportURL(suite.ProxyClient), mockNs, telemetrygen.MetricNames)
		})
	})

})
