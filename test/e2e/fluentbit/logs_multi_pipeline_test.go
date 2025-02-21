//go:build e2e

package fluentbit

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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	Context("When multiple logpipelines exist", Ordered, func() {
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

			backend1 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(ProxyClient)

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
				Build()
			objs = append(objs, &logPipeline1)

			backend2 := backend.New(mockNs, backend.SignalTypeLogs, backend.WithName(backend2Name))
			logProducer := loggen.New(mockNs)
			objs = append(objs, backend2.K8sObjects()...)
			objs = append(objs, logProducer.K8sObject())
			backend2ExportURL = backend2.ExportURL(ProxyClient)

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
				Build()
			objs = append(objs, &logPipeline2)

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
			assert.LogPipelineHealthy(Ctx, K8sClient, pipeline1Name)
			assert.LogPipelineHealthy(Ctx, K8sClient, pipeline2Name)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(ProxyClient, loggen.DefaultName, backend1ExportURL)
			assert.LogsDelivered(ProxyClient, loggen.DefaultName, backend2ExportURL)
		})
	})

})
