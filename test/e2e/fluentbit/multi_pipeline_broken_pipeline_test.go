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
	Context("When a broken logpipeline exists", Ordered, func() {
		var (
			mockNs              = ID()
			healthyPipelineName = IDWithSuffix("healthy")
			brokenPipelineName  = IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeLogs)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(ProxyClient)

			healthyPipeline := testutils.NewLogPipelineBuilder().
				WithName(healthyPipelineName).
				WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
				Build()
			logProducer := loggen.New(mockNs)
			objs = append(objs, logProducer.K8sObject())
			objs = append(objs, &healthyPipeline)

			hostKey := "log-host"
			unreachableHostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData(hostKey, "http://unreachable:9880")).K8sObject()
			brokenPipeline := testutils.NewLogPipelineBuilder().
				WithName(brokenPipelineName).
				WithHTTPOutput(testutils.HTTPHostFromSecret(unreachableHostSecret.Name, unreachableHostSecret.Namespace, hostKey)).
				Build()

			objs = append(objs, &brokenPipeline, unreachableHostSecret)

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
			assert.LogPipelineHealthy(Ctx, K8sClient, healthyPipelineName)
			assert.LogPipelineHealthy(Ctx, K8sClient, brokenPipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(Ctx, K8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(ProxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
