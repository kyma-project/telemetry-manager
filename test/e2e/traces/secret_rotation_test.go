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

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	Context("When tracepipeline referecing a secret with incorrect endpoint exists", Ordered, func() {
		const endpointKey = "traces-endpoint"

		var (
			mockNs       = suite.ID()
			pipelineName = suite.ID()
			backend      *kitbackend.Backend
		)

		// Initially, create a secret with an incorrect endpoint
		secret := kitk8s.NewOpaqueSecret("traces-secret-rotation", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()
		backend = kitbackend.New(mockNs, kitbackend.SignalTypeTraces)

		resources := []client.Object{
			kitk8s.NewNamespace(mockNs).K8sObject(),
			secret.K8sObject(),
			&tracePipeline,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).K8sObject(),
		}
		resources = append(resources, backend.K8sObjects()...)

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(suite.Ctx, resources...)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, resources...)).Should(Succeed())
			})
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.TracePipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should initially not deliver telemetrygen traces to the backend due to the incorrect endpoint in the secret", func() {
			assert.TracesFromNamespacesNotDelivered(suite.ProxyClient, backend.ExportURL(suite.ProxyClient), []string{mockNs})
		})

		It("Should deliver telemetrygen traces to the backend", func() {
			By("Updating secret with the correct endpoint", func() {
				secret.UpdateSecret(kitk8s.WithStringData(endpointKey, backend.Endpoint()))
				Expect(kitk8s.UpdateObjects(suite.Ctx, secret.K8sObject())).Should(Succeed())
			})
			assert.DeploymentReady(suite.Ctx, kitkyma.TraceGatewayName)
			assert.TracePipelineHealthy(suite.Ctx, pipelineName)

			assert.TracesFromNamespaceDelivered(suite.ProxyClient, backend.ExportURL(suite.ProxyClient), mockNs)
		})
	})

})
