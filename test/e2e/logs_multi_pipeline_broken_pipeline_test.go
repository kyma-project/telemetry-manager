//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("When a broken logpipeline exists", Ordered, func() {
		var (
			mockNs              = suite.ID()
			healthyPipelineName = suite.IDWithSuffix("healthy")
			brokenPipelineName  = suite.IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeLogs)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(proxyClient)

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
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			assert.LogPipelineHealthy(ctx, k8sClient, healthyPipelineName)
			assert.LogPipelineHealthy(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSet)
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
