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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
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

			logPipeline1 := kitk8s.NewLogPipelineV1Alpha1(pipeline1Name).
				WithSecretKeyRef(backend1.HostSecretRefV1Alpha1()).
				WithHTTPOutput()
			objs = append(objs, logPipeline1.K8sObject())

			backend2 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend2Name))
			logProducer := loggen.New(mockNs)
			objs = append(objs, backend2.K8sObjects()...)
			objs = append(objs, logProducer.K8sObject())
			backend2ExportURL = backend2.ExportURL(proxyClient)

			logPipeline2 := kitk8s.NewLogPipelineV1Alpha1(pipeline2Name).
				WithSecretKeyRef(backend2.HostSecretRefV1Alpha1()).
				WithHTTPOutput()
			objs = append(objs, logPipeline2.K8sObject())

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

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backend1ExportURL)
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backend2ExportURL)
		})
	})

})
