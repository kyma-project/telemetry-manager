//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("When a broken logpipeline exists", Ordered, func() {
		var (
			mockNs              = suite.IDWithSuffix("broken-pipeline")
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

			healthyPipeline := kitk8s.NewLogPipelineV1Alpha1(healthyPipelineName).
				WithSecretKeyRef(backend.HostSecretRefV1Alpha1()).
				WithHTTPOutput().
				Persistent(suite.IsOperational())
			logProducer := loggen.New(mockNs)
			objs = append(objs, logProducer.K8sObject())
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("log-host", "http://unreachable:9880"))
			brokenPipeline := kitk8s.NewLogPipelineV1Alpha1(brokenPipelineName).
				WithSecretKeyRef(unreachableHostSecret.SecretKeyRefV1Alpha1("log-host")).
				WithHTTPOutput().
				Persistent(suite.IsOperational())

			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

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
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, healthyPipelineName)
			verifiers.LogPipelineShouldBeHealthy(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have a log backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have produced logs in the backend", func() {
			verifiers.LogsShouldBeDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})
	})
})
