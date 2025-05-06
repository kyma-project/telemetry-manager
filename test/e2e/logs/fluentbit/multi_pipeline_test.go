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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsFluentBit), Ordered, func() {
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

			backend1 := backend.New(mockNs, backend.SignalTypeLogsFluentBit, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(suite.ProxyClient)

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithHTTPOutput(testutils.HTTPHost(backend1.Host()), testutils.HTTPPort(backend1.Port())).
				Build()
			objs = append(objs, &logPipeline1)

			backend2 := backend.New(mockNs, backend.SignalTypeLogsFluentBit, backend.WithName(backend2Name))
			logProducer := loggen.New(mockNs)
			objs = append(objs, backend2.K8sObjects()...)
			objs = append(objs, logProducer.K8sObject())
			backend2ExportURL = backend2.ExportURL(suite.ProxyClient)

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithHTTPOutput(testutils.HTTPHost(backend2.Host()), testutils.HTTPPort(backend2.Port())).
				Build()
			objs = append(objs, &logPipeline2)

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
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipeline1Name)
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipeline2Name)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			assert.FluentBitLogsDelivered(suite.ProxyClient, loggen.DefaultName, backend1ExportURL)
			assert.FluentBitLogsDelivered(suite.ProxyClient, loggen.DefaultName, backend2ExportURL)
		})
	})

})
