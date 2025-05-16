//go:build e2e

package application

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

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelSignalPull, suite.LabelExperimental), Ordered, func() {
	Context("When multiple otlp logpipelines exist", Ordered, func() {
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

			backend1 := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(suite.ProxyClient)

			logPipeline1 := testutils.NewLogPipelineBuilder().
				WithName(pipeline1Name).
				WithOTLPInput(false).
				WithApplicationInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backend1.Endpoint())).
				Build()
			objs = append(objs, &logPipeline1)

			backend2 := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithName(backend2Name))
			objs = append(objs, backend2.K8sObjects()...)
			backend2ExportURL = backend2.ExportURL(suite.ProxyClient)

			logPipeline2 := testutils.NewLogPipelineBuilder().
				WithName(pipeline2Name).
				WithOTLPInput(false).
				WithApplicationInput(true).
				WithOTLPOutput(testutils.OTLPEndpoint(backend2.Endpoint())).
				Build()
			objs = append(objs, &logPipeline2)

			logProducer := loggen.New(mockNs)

			objs = append(objs,
				logProducer.K8sObject(),
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
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipeline1Name)
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipeline2Name)
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have a running log agent daemonset", func() {
			assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
		})

		It("Should have running backends", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should verify logs from loggen are delivered", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backend1ExportURL, mockNs)
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backend2ExportURL, mockNs)
		})
	})
})
