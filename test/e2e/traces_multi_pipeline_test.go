//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), Ordered, func() {
	Context("When multiple tracepipelines exist", Ordered, func() {
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

			backend1 := backend.New(mockNs, backend.SignalTypeTraces, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(proxyClient)

			tracePipeline1 := kitk8s.NewTracePipelineV1Alpha1(pipeline1Name).WithOutputEndpointFromSecret(backend1.HostSecretRefV1Alpha1())
			objs = append(objs, tracePipeline1.K8sObject())

			backend2 := backend.New(mockNs, backend.SignalTypeTraces, backend.WithName(backend2Name))
			objs = append(objs, backend2.K8sObjects()...)
			backend2ExportURL = backend2.ExportURL(proxyClient)

			tracePipeline2 := kitk8s.NewTracePipelineV1Alpha1(pipeline2Name).WithOutputEndpointFromSecret(backend2.HostSecretRefV1Alpha1())
			objs = append(objs, tracePipeline2.K8sObject())

			objs = append(objs, telemetrygen.New(mockNs, telemetrygen.SignalTypeTraces).K8sObject())
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
			assert.TracePipelineShouldBeHealthy(ctx, k8sClient, pipeline1Name)
			assert.TracePipelineShouldBeHealthy(ctx, k8sClient, pipeline2Name)
		})
		It("Should have a trace backend running", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})
		It("Should verify traces from telemetrygen are delivered", func() {
			assert.TracesFromNamespaceShouldBeDelivered(proxyClient, backend1ExportURL, mockNs)
			assert.TracesFromNamespaceShouldBeDelivered(proxyClient, backend2ExportURL, mockNs)
		})
	})
})
